/* =========================================================================
   MicroVT  --  Lightweight VT100 + Kitty Graphics browser terminal lib
   =========================================================================

   Constructor:
     new MicroVT(element, cols, rows, opts)
       opts.bg            -- default background CSS colour (default '#000000')
       opts.fg            -- default foreground CSS colour (default '#cccccc')
       opts.maxScrollback -- scrollback line limit     (default 3000)
       opts.onScroll(top, max) -- called when scrollTop changes
       opts.onImgChange(count) -- called when image list changes

   Methods:
     .write(string)      -- ingest raw terminal data (UTF-16 string)
     .scrollBy(delta)    -- scroll scrollback +-delta rows  (0 = at bottom)
     .scrollToTop()
     .scrollToBottom()
     .clear()            -- ESC[2J ESC[H
     .dispose()          -- clean up DOM

   Read-only:
     .cols, .rows, .cw (cell px width), .ch (cell px height)
     .imageCount
     .scrollTop, .scrollback.length
   ========================================================================= */

// Terminal implementation
//
class MicroVT {
  /* --- construction --- */
  constructor(el, cols = 80, rows = 24, opts = {}) {
    this.root = el;
    this.cols = cols;
    this.rows = rows;
    this.opts = { bg: '#000000', fg: '#cccccc', maxScrollback: 3000, ...opts };

    this.defaultBg = this.opts.bg;
    this.defaultFg = this.opts.fg;

    this._pal256 = this._build256();

    this._buildDOM();
    this._measureCell();
    this._resetState();
    this._bindInput();
  }

  /* --- DOM --- */
  //
  _buildDOM() {
    this.root.innerHTML = '';
    this.root.style.cssText =
      "font-family:'Courier New',monospace;font-size:15px;line-height:1;";

    this._layerG = Object.assign(document.createElement('div'), { className: 'mvt-layer-g' });
    this._layerT = Object.assign(document.createElement('div'), { className: 'mvt-layer-t' });
    this.root.append(this._layerG, this._layerT);

    this._rowEls = Array.from({ length: this.rows }, () => {
      const d = Object.assign(document.createElement('div'), { className: 'mvt-row' });
      this._layerT.appendChild(d);
      return d;
    });
  }

  _measureCell() {
    const m = document.createElement('span');
    m.style.cssText =
      "display:inline-block;white-space:pre;font-family:'Courier New',monospace;font-size:15px;line-height:1;";
    m.textContent = 'M';
    this._layerT.appendChild(m);
    const r = m.getBoundingClientRect();
    this._layerT.removeChild(m);
    this.cw = Math.ceil(r.width);
    this.ch = Math.ceil(r.height);
    this.root.style.width      = `${this.cols * this.cw}px`;
    this.root.style.height     = `${this.rows * this.ch}px`;
    this.root.style.lineHeight = `${this.ch}px`;
    this._rowEls.forEach(d => { d.style.height = d.style.lineHeight = `${this.ch}px`; });
  }

  /* --- state --- */
  //
  _defAttr() {
    return { fg: this.defaultFg, bg: this.defaultBg,
             b: false, i: false, u: false, inv: false, s: false, blink: false };
  }

  _makeRow() {
    const def = this._defAttr();
    return Array.from({ length: this.cols }, () => ({ c: ' ', ...def }));
  }

  _resetState() {
    this.cx = 0; this.cy = 0;
    this._attr  = this._defAttr();
    this._saved = { x: 0, y: 0, attr: this._defAttr() };

    // scrollback: oldest = [0], newest = [length-1]
    this.scrollback = [];
    // 0 = viewing live bottom; N = N rows into scrollback
    this.scrollTop  = 0;

    // active grid
    this._grid = Array.from({ length: this.rows }, () => this._makeRow());

    // images: { id, el, x, y(grid row), cols, rows }
    this._images = [];
    this._nextId = 1;

    // accumulates data across chunked Kitty APC transfers
    this._kBuf = null;

    // parser FSM: 0=normal 1=ESC 2=CSI 3=APC 4=APC_ESC
    this._ps = 0;
    this._pb = '';
  }

  /* --- input bindings --- */
  //
  _bindInput() {
    this.root.addEventListener('wheel', e => {
      e.preventDefault();
      // deltaY > 0 means finger/wheel moving down, which scrolls content up
      // into scrollback; deltaY < 0 returns toward the live view
      this.scrollBy(-Math.sign(e.deltaY) * 3);
    }, { passive: false });

    this.root.addEventListener('keydown', e => {
      if (e.key === 'PageUp')               { this.scrollBy( this.rows); e.preventDefault(); }
      if (e.key === 'PageDown')             { this.scrollBy(-this.rows); e.preventDefault(); }
      if (e.key === 'Home' && e.ctrlKey)    { this.scrollToTop();        e.preventDefault(); }
      if (e.key === 'End'  && e.ctrlKey)    { this.scrollToBottom();     e.preventDefault(); }
    });
  }

  /* --- scrollback API --- */
  //
  scrollBy(delta) {
    this.scrollTop = Math.max(0, Math.min(this.scrollback.length, this.scrollTop + delta));
    this._render();
    this.opts.onScroll?.(this.scrollTop, this.scrollback.length);
  }
  scrollToTop()    { this.scrollTop = this.scrollback.length; this._render(); }
  scrollToBottom() { this.scrollTop = 0;                      this._render(); }

  /* --- internal line-feed and scrollback push --- */
  //
  _lineFeed() {
    this.cy++;
    if (this.cy < this.rows) return;
    this.cy = this.rows - 1;

    // evict top row to scrollback
    const evicted = this._grid.shift();
    this.scrollback.push(evicted);
    if (this.scrollback.length > this.opts.maxScrollback) this.scrollback.shift();
    this._grid.push(this._makeRow());

    // keep the viewport locked to the same scrollback position
    if (this.scrollTop > 0)
      this.scrollTop = Math.min(this.scrollTop + 1, this.scrollback.length);

    // y is in live-grid space (0 = top of live grid). An image that scrolls
    // above the live grid (y < 0) is in scrollback territory and stays alive
    // until it has scrolled past the entire scrollback buffer.
    for (let i = this._images.length - 1; i >= 0; i--) {
      this._images[i].y--;
      if (this._images[i].y + this._images[i].rows < -this.scrollback.length) {
        this._images[i].el.remove();
        this._images.splice(i, 1);
      }
    }
    this.opts.onImgChange?.(this._images.length);
  }

  /* --- write --- */
  //
  write(data) {
    for (let i = 0; i < data.length; i++) {
      const c = data[i], code = data.charCodeAt(i);

      switch (this._ps) {
        // normal character or C0 control
        case 0:
          if      (code === 0x1B)                  this._ps = 1;
          else if (code >= 0x20)                   this._putChar(c);
          else if (c==='\n'||c==='\v'||c==='\f')   this._lineFeed();
          else if (c === '\r')                     this.cx = 0;
          else if (c === '\b')                     this.cx = Math.max(0, this.cx-1);
          else if (c === '\t')                     this.cx = (this.cx+8)&~7;
          break;

        // ESC dispatch
        case 1:
          if      (c === '[') { this._ps = 2; this._pb = ''; }
          else if (c === '_') { this._ps = 3; this._pb = ''; }
          else if (c === 'D') { this._lineFeed(); this._ps = 0; }
          else if (c === 'M') { this.cy = Math.max(0, this.cy-1); this._ps = 0; }
          else if (c === '7') { this._saved = { x:this.cx, y:this.cy, attr:{...this._attr} }; this._ps = 0; }
          else if (c === '8') { this.cx = this._saved.x; this.cy = this._saved.y; this._attr = {...this._saved.attr}; this._ps = 0; }
          else                  this._ps = 0;
          break;

        // CSI parameter accumulation
        case 2:
          if ((code>=0x20&&code<=0x3F)||c===';') this._pb += c;
          else { this._csi(c); this._ps = 0; }
          break;

        // APC body accumulation
        case 3:
          if (code === 0x1B) this._ps = 4;
          else               this._pb += c;
          break;

        // APC terminator: ESC followed by '\' closes the sequence
        case 4:
          if (c === '\\') { this._kittyAPC(this._pb); this._ps = 0; this._pb = ''; }
          else            { this._pb += '\x1b'+c;     this._ps = 3; }
          break;
      }
    }
    this._render();
  }

  /* --- cell put --- */
  //
  _putChar(c) {
    if (this.cx >= this.cols) { this.cx = 0; this._lineFeed(); }
    this._grid[this.cy][this.cx] = { c, ...this._attr };
    this.cx++;
  }

  /* --- CSI dispatch --- */
  //
  _csi(cmd) {
    const a = this._pb.split(';').map(x => parseInt(x)||0);
    const n = a[0]||1;
    switch (cmd) {
      case 'A': this.cy = Math.max(0,          this.cy-n); break;
      case 'B': this.cy = Math.min(this.rows-1, this.cy+n); break;
      case 'C': this.cx = Math.min(this.cols-1, this.cx+n); break;
      case 'D': this.cx = Math.max(0,           this.cx-n); break;
      case 'E': this.cy = Math.min(this.rows-1, this.cy+n); this.cx = 0; break;
      case 'F': this.cy = Math.max(0,           this.cy-n); this.cx = 0; break;
      case 'G': this.cx = Math.min(this.cols-1, Math.max(0, n-1)); break;
      case 'H': case 'f':
        this.cy = Math.min(this.rows-1, Math.max(0, (a[0]||1)-1));
        this.cx = Math.min(this.cols-1, Math.max(0, (a[1]||1)-1)); break;
      case 'J': this._eraseDisp(a[0]); break;
      case 'K': this._eraseLine(a[0]); break;
      case 'L': this._insertLines(n); break;
      case 'M': this._deleteLines(n); break;
      case 'P': this._deleteChars(n); break;
      case '@': this._insertChars(n); break;
      case 'm': this._sgr(a); break;
    }
  }

  _eraseDisp(m) {
    const def = this._defAttr();
    const clr = row => row.forEach(c => { c.c = ' '; Object.assign(c, def); });
    if (m===2||m===3) {
      this._grid.forEach(clr);
      this._images.forEach(img => img.el.remove()); this._images = [];
      this.cx = 0; this.cy = 0;
    } else if (m===1) {
      for (let y = 0; y < this.cy; y++) clr(this._grid[y]);
      for (let x = 0; x <= this.cx; x++) Object.assign(this._grid[this.cy][x], { c:' ', ...def });
    } else {
      for (let x = this.cx; x < this.cols; x++) Object.assign(this._grid[this.cy][x], { c:' ', ...def });
      for (let y = this.cy+1; y < this.rows; y++) clr(this._grid[y]);
    }
  }

  _eraseLine(m) {
    const def = this._defAttr(), row = this._grid[this.cy];
    if (m===1)      for (let x = 0; x <= this.cx; x++)        Object.assign(row[x], { c:' ', ...def });
    else if (m===2) row.forEach(c => { c.c = ' '; Object.assign(c, def); });
    else            for (let x = this.cx; x < this.cols; x++) Object.assign(row[x], { c:' ', ...def });
  }

  _insertLines(n) {
    for (let i = 0; i < n; i++) {
      this._grid.splice(this.cy, 0, this._makeRow());
      if (this._grid.length > this.rows) this._grid.pop();
    }
  }
  _deleteLines(n) {
    for (let i = 0; i < n; i++) {
      this._grid.splice(this.cy, 1);
      this._grid.push(this._makeRow());
    }
  }
  _deleteChars(n) {
    const row = this._grid[this.cy];
    row.splice(this.cx, n);
    while (row.length < this.cols) row.push({ c:' ', ...this._defAttr() });
  }
  _insertChars(n) {
    const row = this._grid[this.cy];
    for (let i = 0; i < n; i++) row.splice(this.cx, 0, { c:' ', ...this._defAttr() });
    while (row.length > this.cols) row.pop();
  }

  /* --- SGR --- */
  //
  _sgr(a) {
    if (!a.length) a = [0];
    for (let i = 0; i < a.length; i++) {
      const v = a[i];
      if      (v === 0)  this._attr = this._defAttr();
      else if (v === 1)  this._attr.b     = true;
      else if (v === 3)  this._attr.i     = true;
      else if (v === 4)  this._attr.u     = true;
      else if (v === 5)  this._attr.blink = true;
      else if (v === 7)  this._attr.inv   = true;
      else if (v === 9)  this._attr.s     = true;
      else if (v === 22) this._attr.b     = false;
      else if (v === 23) this._attr.i     = false;
      else if (v === 24) this._attr.u     = false;
      else if (v === 25) this._attr.blink = false;
      else if (v === 27) this._attr.inv   = false;
      else if (v === 29) this._attr.s     = false;
      else if (v >= 30  && v <= 37)  this._attr.fg = this._pal256[v-30];
      else if (v === 38) {
        if      (a[i+1]===5) { this._attr.fg = this._pal256[a[i+2]]; i+=2; }
        else if (a[i+1]===2) { this._attr.fg = `rgb(${a[i+2]},${a[i+3]},${a[i+4]})`; i+=4; }
      }
      else if (v === 39)              this._attr.fg = this.defaultFg;
      else if (v >= 40  && v <= 47)  this._attr.bg = this._pal256[v-40];
      else if (v === 48) {
        if      (a[i+1]===5) { this._attr.bg = this._pal256[a[i+2]]; i+=2; }
        else if (a[i+1]===2) { this._attr.bg = `rgb(${a[i+2]},${a[i+3]},${a[i+4]})`; i+=4; }
      }
      else if (v === 49)              this._attr.bg = this.defaultBg;
      else if (v >= 90  && v <= 97)  this._attr.fg = this._pal256[v-90+8];
      else if (v >= 100 && v <= 107) this._attr.bg = this._pal256[v-100+8];
    }
  }

  /* --- Kitty Graphics Protocol ---
     APC format: ESC _ G<params>;<base64_chunk> ESC \
     Params: a=T|d, f=32|24, s=width, v=height, c=cols, r=rows, m=0|1, i=id */
  //
  _kittyAPC(rawPayload) {
    // Kitty payloads start with 'G' as the protocol identifier; strip it
    if (!rawPayload.startsWith('G')) return;
    const payload = rawPayload.slice(1);

    const semi  = payload.indexOf(';');
    const pstr  = semi >= 0 ? payload.slice(0, semi) : payload;
    const data  = semi >= 0 ? payload.slice(semi+1)  : '';

    const params = {};
    pstr.split(',').forEach(kv => {
      const eq = kv.indexOf('=');
      if (eq > 0) params[kv.slice(0,eq)] = kv.slice(eq+1);
    });

    const more = (params.m || '0') === '1';

    // first chunk carries all metadata; subsequent chunks only carry data
    if (!this._kBuf) {
      this._kBuf = { params, data };
    } else {
      this._kBuf.data += data;
    }
    if (more) return;

    // final chunk -- dispatch
    const buf = this._kBuf;
    this._kBuf = null;
    const action = buf.params.a || 'T';

    if (action === 'd') {
      // d=a deletes all images; i=N deletes by id
      const byId = parseInt(buf.params.i || '0');
      const all  = buf.params.d === 'a' || byId === 0;
      this._images = this._images.filter(img => {
        if (all || img.id === byId) { img.el.remove(); return false; }
        return true;
      });
      this.opts.onImgChange?.(this._images.length);
      return;
    }

    const b64 = buf.data.replace(/\s/g, '');
    if (!b64) return;

    const fmt   = parseInt(buf.params.f || '32');
    const sw    = parseInt(buf.params.s || '0');
    const sh    = parseInt(buf.params.v || '0');
    const gCols = parseInt(buf.params.c || '0');
    const gRows = parseInt(buf.params.r || '0');
    // explicit id or auto-increment
    const imgId = parseInt(buf.params.i || String(this._nextId++));

    this._kittyDisplay(b64, fmt, sw, sh, gCols, gRows, imgId);
  }

  _kittyDisplay(b64, fmt, sw, sh, gCols, gRows, imgId) {
    let binary;
    try {
      const raw = atob(b64);
      binary = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) binary[i] = raw.charCodeAt(i);
    } catch(e) { console.warn('MicroVT Kitty b64 error', e); return; }

    const bpp = fmt===24 ? 3 : 4;
    let W = sw, H = sh;
    if (!W||!H) {
      const tot = binary.length/bpp;
      W = Math.max(1, Math.round(Math.sqrt(tot)));
      H = Math.ceil(tot/W);
    }

    // decode pixel data into a canvas, then snapshot as a data URL
    const canvas = document.createElement('canvas');
    canvas.width = W; canvas.height = H;
    const ctx = canvas.getContext('2d');
    const id  = ctx.createImageData(W, H);
    const d   = id.data;
    for (let p = 0, j = 0; p < binary.length; p += bpp, j++) {
      const o = j*4;
      d[o]   = binary[p];
      d[o+1] = binary[p+1];
      d[o+2] = binary[p+2];
      d[o+3] = fmt===24 ? 255 : binary[p+3];
    }
    ctx.putImageData(id, 0, 0);

    const dispCols = gCols || Math.ceil(W/this.cw);
    const dispRows = gRows || Math.ceil(H/this.ch);

    // Snapshot placement position before advancing the cursor. The image
    // record is pushed only after the line-feeds, so _lineFeed's scroll-
    // images loop never touches this freshly placed image.
    const placeX = this.cx;
    const placeY = this.cy;

    // Advance cursor first. _lineFeed may scroll the grid, shifting existing
    // images' .y values, but our new image is not in the list yet.
    for (let i = 0; i < dispRows; i++) this._lineFeed();
    this.cx = 0;

    // Reconstruct the correct post-scroll grid row. After dispRows line-feeds,
    // cy is clamped to rows-1, and the image top sits at (placeY - scrollDelta).
    const scrolledBy = Math.max(0, placeY + dispRows - (this.rows - 1));
    const finalY     = placeY - scrolledBy;

    const img = Object.assign(document.createElement('img'), { className: 'mvt-img' });
    img.src = canvas.toDataURL();
    img.style.cssText =
      `width:${dispCols*this.cw}px;height:${dispRows*this.ch}px;left:${placeX*this.cw}px;z-index:10;`;

    const rec = { id: imgId, el: img, x: placeX, y: finalY, cols: dispCols, rows: dispRows };
    this._images.push(rec);
    this._layerG.appendChild(img);
    this.opts.onImgChange?.(this._images.length);
  }

  /* --- render --- */
  //
  _render() {
    const sbLen  = this.scrollback.length;
    // how many scrollback rows are visible at the top of the screen
    const sbShow = Math.min(this.scrollTop, sbLen);

    for (let r = 0; r < this.rows; r++) {
      const rowData = r < sbShow
        ? this.scrollback[sbLen - sbShow + r]
        : this._grid[r - sbShow];
      this._renderRow(r, rowData);
    }

    // images live in grid-row space; when scrolled, grid starts at screen row sbShow
    this._images.forEach(img => {
      const screenY = img.y + sbShow;
      img.el.style.top     = `${screenY * this.ch}px`;
      img.el.style.display = (screenY >= 0 && screenY < this.rows) ? '' : 'none';
    });
  }

  _renderRow(r, row) {
    const el = this._rowEls[r];
    if (!row) { el.innerHTML = ''; return; }

    // run-length merge: build one <span> per attribute run to minimise DOM nodes
    let html = '';
    let span = { ...row[0], text: '' };

    for (let x = 0; x < this.cols; x++) {
      const cell = row[x] ?? { c:' ', ...this._defAttr() };
      if (cell.fg    !== span.fg    || cell.bg   !== span.bg    ||
          cell.b     !== span.b     || cell.i    !== span.i     ||
          cell.u     !== span.u     || cell.inv  !== span.inv   ||
          cell.s     !== span.s     || cell.blink !== span.blink) {
        html += this._spanHTML(span);
        span  = { ...cell, text: '' };
      }
      span.text += cell.c;
    }
    html += this._spanHTML(span);
    // skip DOM write when content has not changed
    if (el.innerHTML !== html) el.innerHTML = html;
  }

  _spanHTML(s) {
    if (!s.text) return '';
    const txt = s.text.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    let f = s.fg, b = s.bg;
    if (s.inv) [f, b] = [b, f];
    let sty = '';
    if (f !== this.defaultFg) sty += `color:${f};`;
    if (b !== this.defaultBg) sty += `background:${b};`;
    const cls = [];
    if (s.b)     cls.push('vt-b');
    if (s.i)     cls.push('vt-i');
    if (s.u)     cls.push('vt-u');
    if (s.s)     cls.push('vt-s');
    if (s.blink) cls.push('vt-blnk');
    const c = cls.length ? ` class="${cls.join(' ')}"` : '';
    const t = sty        ? ` style="${sty}"`           : '';
    return `<span${c}${t}>${txt}</span>`;
  }

  /* --- colour palettes --- */
  //
  _build256() {
    const p = [
      '#000000','#cd0000','#00cd00','#cdcd00','#0000ee','#cd00cd','#00cdcd','#e5e5e5',
      '#7f7f7f','#ff0000','#00ff00','#ffff00','#5c5cff','#ff00ff','#00ffff','#ffffff'
    ];
    // 6x6x6 RGB cube: indices 16-231
    const l = [0, 95, 135, 175, 215, 255];
    for (let r = 0; r < 6; r++)
      for (let g = 0; g < 6; g++)
        for (let b = 0; b < 6; b++) p.push(`rgb(${l[r]},${l[g]},${l[b]})`);
    // 24-step grayscale ramp: indices 232-255
    for (let i = 0; i < 24; i++) { const v = 8+i*10; p.push(`rgb(${v},${v},${v})`); }
    return p;
  }

  /* --- public helpers --- */
  //
  clear()   { this.write('\x1b[2J\x1b[H'); }
  dispose() { this.root.innerHTML = ''; }
  get imageCount() { return this._images.length; }
}

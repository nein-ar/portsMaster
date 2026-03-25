// Initialisation
//
document.addEventListener('DOMContentLoaded', () => {
    const termEl = document.getElementById('viewer-term');
    if (!termEl) return;

    const term = new MicroVT(termEl, 80, 24, {
        bg: '#000',
        fg: '#c8cdd8'
    });

    // Content loading
    //
    let data = null;
    try {
        data = JSON.parse(document.getElementById('embedded-content').textContent);
    } catch (e) {
        term.write('error: failed to load embedded content\r\n');
        return;
    }

    if (!data || !data.filename) {
        term.write('error: invalid embedded data\r\n');
        return;
    }

    // Log handling
    //
    if (data.filename === 'ci_log.txt') {
        renderEmbeddedLog(term, data.content || '');
    }
});

// Log rendering
//
function renderEmbeddedLog(term, content) {
    const formatted = content.replace(/\n/g, '\r\n');
    term.write(formatted);
    term.write('\r\n');
}

// Kitty format helper
//
function pixelsToKitty(rgba, w, h, cols, rows, chunkSize = 4096) {
    const b64 = btoa(String.fromCharCode(...rgba));
    let out = '';
    for (let off = 0; off < b64.length; off += chunkSize) {
        const chunk = b64.slice(off, off + chunkSize);
        const more  = (off + chunkSize < b64.length) ? 1 : 0;
        const p = off === 0
            ? `a=T,f=32,s=${w},h=${h},c=${cols},r=${rows},m=${more}`
            : `m=${more}`;
        out += `\x1b_G${p};${chunk}\x1b\\`;
    }
    return out;
}

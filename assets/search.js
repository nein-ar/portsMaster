document.addEventListener('DOMContentLoaded', () => {
    const advancedForm = document.getElementById('advanced-search-form');
    const form = advancedForm || document.querySelector('.search-box');
    
    if (!form) return;

    const resultsContainer = advancedForm ? document.getElementById('search-results') : null;
    const queryInput = form.querySelector('input[name="q"]');
    const catSelect = form.querySelector('select[name="cat"]');
    const licSelect = form.querySelector('select[name="lic"]');
    
    let index = null;
    let loadingPromise = null;
    let debounceTimer;
    
    const app = document.getElementById('search-app');
    const body = document.body;
    const baseUrl = body.dataset.baseUrl || '';
    const portsUrl = (app && app.dataset.portsUrl) || (baseUrl + '/ports.json');

    function loadIndex() {
        if (index) return Promise.resolve(index);
        if (loadingPromise) return loadingPromise;
        loadingPromise = fetch(portsUrl).then(r => r.json()).then(data => { index = data; return data; });
        return loadingPromise;
    }

    function checkUrlParams() {
        const params = new URLSearchParams(window.location.search);
        const q = params.get('q');
        const cat = params.get('cat');
        const lic = params.get('lic');
        
        let val = q || '';
        if (cat) {
             const tag = `category:${cat}`;
             if (!val.includes(tag)) val = val ? `${val} && ${tag}` : tag;
        }
        if (lic) {
            const tag = `license:${lic}`;
            if (!val.includes(tag)) val = val ? `${val} && ${tag}` : tag;
        }

        if (val) {
            queryInput.value = val;
            performSearch();
        }
    }

    if (advancedForm || window.location.search) {
        loadIndex().then(checkUrlParams);
    }

    queryInput.addEventListener('focus', loadIndex);
    queryInput.addEventListener('input', () => {
        if (!advancedForm) return;
        loadIndex().then(() => {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(performSearch, 300);
        });
    });

    form.addEventListener('submit', (e) => {
        if (!advancedForm) return; // Natural submit for global search
        e.preventDefault();
        loadIndex().then(() => {
            performSearch();
            const url = new URL(window.location);
            url.searchParams.set('q', queryInput.value);
            url.searchParams.delete('cat');
            url.searchParams.delete('lic');
            window.history.pushState({}, '', url);
        });
    });

    function handleDropdown(select, prefix) {
        if (!select) return;
        select.addEventListener('change', () => {
            const val = select.value;
            if (!val) return;
            if (!advancedForm) { form.submit(); return; }
            loadIndex(); 
            let query = queryInput.value;
            const tag = `${prefix}:${val}`;
            const regex = new RegExp(`${prefix}:\\S+`, 'g');
            let cleanQuery = query.replace(regex, '').replace(/&&\\s*$/, '').replace(/^&&\\s*/, '').trim();
            queryInput.value = cleanQuery.length === 0 ? `${tag} && ` : `${cleanQuery} && ${tag}`;
            performSearch();
            select.value = "";
        });
    }

    handleDropdown(catSelect, 'category');
    handleDropdown(licSelect, 'license');

    function performSearch() {
        if (!index || !resultsContainer) return;
        const rawQuery = queryInput.value.trim();
        if (!rawQuery) {
            resultsContainer.innerHTML = '';
            return;
        }
        if (rawQuery === '*') {
            renderResults(index);
            return;
        }
        const results = index.filter(p => evaluateQuery(p, rawQuery));
        renderResults(results);
    }

    function evaluateQuery(p, query) {
        const orGroups = query.split(/\\|\\|\\s+OR\\s+/i);
        return orGroups.some(group => {
            const trimmedGroup = group.trim();
            if (!trimmedGroup) return false;
            return evaluateAndGroup(p, trimmedGroup);
        });
    }

    function evaluateAndGroup(p, groupQuery) {
        const tokens = [];
        const regex = /"([^"]+)"|(\S+)/g;
        let match;
        while ((match = regex.exec(groupQuery)) !== null) {
            const token = match[1] || match[2];
            if (token === '&&' || token.toUpperCase() === 'AND') continue;
            tokens.push(token);
        }
        return tokens.every(token => matchToken(p, token));
    }

    function matchToken(p, token) {
        let text = token.toLowerCase();
        let invert = false;
        if (text.startsWith('!')) { invert = true; text = text.substring(1); }

        let match = false;
        if (text.startsWith('name:')) match = p.n.toLowerCase().includes(text.substring(5));
        else if (text.startsWith('author:')) match = p.a && p.a.toLowerCase().includes(text.substring(7));
        else if (text.startsWith('category:')) match = p.c.toLowerCase() === text.substring(9);
        else if (text.startsWith('license:')) match = p.l && p.l.toLowerCase().includes(text.substring(8));
        else if (text.startsWith('provides:')) match = p.pds && p.pds.some(x => x.toLowerCase().includes(text.substring(9)));
        else if (text.startsWith('depends:')) match = p.dps && p.dps.some(x => x.toLowerCase().includes(text.substring(8)));
        else if (text === 'is:broken') match = p.br;
        else if (text === 'is:unmaintained') match = p.un;
        else if (text === 'is:new') {
            const monthAgo = (Date.now() / 1000) - (30 * 24 * 60 * 60);
            match = p.dt > monthAgo;
        }
        else if (text === 'is:updated') {
            const weekAgo = (Date.now() / 1000) - (7 * 24 * 60 * 60);
            match = p.dt > weekAgo;
        }
        else if (text.startsWith('since:')) {
            const val = text.substring(6);
            const num = parseInt(val);
            const unit = val.slice(-1);
            const now = Date.now() / 1000;
            let limit = 0;
            if (unit === 'd') limit = now - (num * 24 * 3600);
            else if (unit === 'w') limit = now - (num * 7 * 24 * 3600);
            else if (unit === 'y') limit = now - (num * 365 * 24 * 3600);
            else limit = now - (num * 24 * 3600); // default to days
            match = p.dt > limit;
        }
        else {
            match = p.n.toLowerCase().includes(text) || (p.d && p.d.toLowerCase().includes(text)) || (p.c && p.c.toLowerCase().includes(text));
        }
        return invert ? !match : match;
    }

    function renderResults(results) {
        if (results.length === 0) {
            resultsContainer.innerHTML = '<div class="result-item">No matches found.</div>';
            return;
        }

        const rows = results.slice(0, 100).map(p => `
            <tr>
                <td>
                    <div class="flex-center">
                        <span class="status-indicator ${p.br ? 'broken' : 'ok'}"></span>
                        <a href="${baseUrl}/ports/${p.c}/${p.n}/index.html" class="res-name ${p.br ? 'status-broken' : ''}">${p.n}</a>
                    </div>
                </td>
                <td class="res-ver">${p.v}</td>
                <td class="res-cat">/${p.c}</td>
                <td class="res-desc">
                    ${p.d || ''}
                    ${p.un ? '<span class="status-unmaintained ml-10">unmaintained</span>' : ''}
                </td>
            </tr>
        `).join('');

        resultsContainer.innerHTML = `
            <div class="search-results-container">
                <div class="search-results-header">
                    Found ${results.length} results
                </div>
                <table class="search-results-table">
                    <thead>
                        <tr>
                            <th style="width: 20%">Port</th>
                            <th style="width: 15%">Version</th>
                            <th style="width: 15%">Category</th>
                            <th>Description</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${rows}
                    </tbody>
                </table>
            </div>
        `;
    }
});

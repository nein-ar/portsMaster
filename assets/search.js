document.addEventListener('DOMContentLoaded', () => {
    const searchForm = document.getElementById('searchForm');
    if (!searchForm) return;

    const queryInput = document.getElementById('query');
    const resultsDiv = document.getElementById('results');
    const resultsBody = document.getElementById('resultsBody');
    const resultCount = document.getElementById('resultCount');
    const queryDisplay = document.getElementById('queryDisplay');
    const searchTimeDisplay = document.getElementById('searchTime');

    const searchName = document.getElementById('search_name');
    const searchDesc = document.getElementById('search_desc');
    const searchFiles = document.getElementById('search_files');
    const searchDeps = document.getElementById('search_deps');
    const filterCategory = document.getElementById('filter-category');

    let portsData = [];
    const baseUrl = document.body.dataset.baseUrl || '';

    // Load ports data
    fetch(baseUrl + '/ports.json')
        .then(r => r.json())
        .then(data => {
            portsData = data || [];
            // Check for query in URL
            const urlParams = new URLSearchParams(window.location.search);
            const q = urlParams.get('query') || urlParams.get('q');
            if (q) {
                queryInput.value = q;
                performSearch(q);
            }
        });

    searchForm.addEventListener('submit', (e) => {
        e.preventDefault();
        performSearch(queryInput.value);
        // Update URL
        const url = new URL(window.location);
        url.searchParams.set('q', queryInput.value);
        window.history.pushState({}, '', url);
    });

    queryInput.addEventListener('input', () => {
        const q = queryInput.value.trim();
        if (q.length > 2 || q === '*') {
            performSearch(q);
        }
    });

    function performSearch(query) {
        const startTime = performance.now();
        const rawQuery = query.toLowerCase().trim();
        
        if (!rawQuery) {
            resultsDiv.classList.add('display-none');
            return;
        }

        let filtered = portsData;

        if (rawQuery !== '*') {
            filtered = portsData.filter(p => evaluateQuery(p, rawQuery));
        }

        // Global category filter from dropdown
        if (filterCategory.value) {
            filtered = filtered.filter(p => p.c === filterCategory.value);
        }

        const endTime = performance.now();
        displayResults(filtered, query, Math.round(endTime - startTime));
    }

    function evaluateQuery(p, query) {
        // Support logical OR (||)
        const orGroups = query.split(/\|\||\s+OR\s+/i);
        return orGroups.some(group => {
            const trimmedGroup = group.trim();
            if (!trimmedGroup) return false;
            // Support logical AND (&&) within groups
            const tokens = trimmedGroup.split(/&&|\s+AND\s+/i);
            return tokens.every(token => matchToken(p, token.trim()));
        });
    }

    function matchToken(p, token) {
        if (!token) return true;
        let text = token;
        let invert = false;
        if (text.startsWith('!')) { invert = true; text = text.substring(1); }
        if (text.startsWith('NOT ')) { invert = true; text = text.substring(4); }

        let match = false;
        if (text.startsWith('name:')) match = p.n.toLowerCase().includes(text.substring(5));
        else if (text.startsWith('desc:') || text.startsWith('description:')) match = p.d.toLowerCase().includes(text.substring(text.indexOf(':') + 1));
        else if (text.startsWith('cat:') || text.startsWith('category:')) match = p.c.toLowerCase() === text.substring(text.indexOf(':') + 1);
        else if (text.startsWith('dep:') || text.startsWith('depends:')) match = p.dps && p.dps.some(d => d.toLowerCase().includes(text.substring(text.indexOf(':') + 1)));
        else if (text.startsWith('provides:')) match = p.pds && p.pds.some(f => f.toLowerCase().includes(text.substring(9)));
        else if (text === 'is:broken') match = p.br;
        else if (text === 'is:unmaintained') match = p.un;
        else if (text === 'is:new') {
            const thirtyDaysAgo = (Date.now() / 1000) - (30 * 86400000 / 1000);
            match = p.dt > thirtyDaysAgo;
        }
        else if (text === 'is:updated') {
            const sevenDaysAgo = (Date.now() / 1000) - (7 * 86400000 / 1000);
            match = p.dt > sevenDaysAgo;
        }
        else if (text.startsWith('since:')) {
            const val = text.substring(6);
            const num = parseInt(val);
            const unit = val.slice(-1);
            const now = Date.now() / 1000;
            let limit = 0;
            if (unit === 'd') limit = now - (num * 24 * 3600);
            else if (unit === 'w') limit = now - (num * 7 * 24 * 3600);
            else if (unit === 'm') limit = now - (num * 30 * 24 * 3600);
            else limit = now - (num * 24 * 3600);
            match = p.dt > limit;
        }
        else {
            // Standard search across multiple fields
            let fieldsMatch = false;
            if (searchName.checked && p.n.toLowerCase().includes(text)) fieldsMatch = true;
            if (searchDesc.checked && p.d.toLowerCase().includes(text)) fieldsMatch = true;
            if (searchFiles.checked && p.pds?.some(f => f.toLowerCase().includes(text))) fieldsMatch = true;
            if (searchDeps.checked && p.dps?.some(d => d.toLowerCase().includes(text))) fieldsMatch = true;
            match = fieldsMatch;
        }

        return invert ? !match : match;
    }

    function displayResults(results, query, time) {
        resultsDiv.classList.remove('display-none');
        resultsDiv.classList.add('display-block');
        resultCount.textContent = results.length;
        queryDisplay.textContent = query;
        searchTimeDisplay.textContent = time;
        resultsBody.innerHTML = '';

        results.slice(0, 200).forEach(p => {
            const row = document.createElement('tr');
            const portUrl = baseUrl + '/ports/' + p.c + '/' + p.n + '/index.html';
            const catUrl = baseUrl + '/categories/' + p.c + '/index.html';
            
            let statusClass = 'none';
            if (p.st === 'success') statusClass = 'success';
            else if (p.st === 'failed') statusClass = 'failed';
            else if (p.br) statusClass = 'failed';

            row.innerHTML = `
                <td>
                    <div class="flex-center">
                        <span class="status-dot ${statusClass}"></span>
                        <a href="${portUrl}" class="port-name">${p.n}</a>
                    </div>
                </td>
                <td class="version">${p.v}</td>
                <td><a href="${catUrl}">/${p.c}</a></td>
                <td>
                    ${highlightMatch(p.d, query)}
                    ${p.un ? '<span class="status-unmaintained ml-10">unmaintained</span>' : ''}
                </td>
            `;
            resultsBody.appendChild(row);
        });

        if (results.length > 200) {
            const moreRow = document.createElement('tr');
            moreRow.innerHTML = `<td colspan="4" class="search-results-info">Showing first 200 of ${results.length} results. Refine your search for more.</td>`;
            resultsBody.appendChild(moreRow);
        }
    }

    function highlightMatch(text, query) {
        if (!query || query.includes(':') || query === '*') return text;
        const words = query.split(/\s+/).filter(w => w.length > 2 && !['and', 'or', 'not'].includes(w.toLowerCase()));
        if (words.length === 0) return text;
        
        let highlighted = text;
        words.forEach(word => {
            const regex = new RegExp(`(${word})`, 'gi');
            highlighted = highlighted.replace(regex, '<span class="match-highlight">$1</span>');
        });
        return highlighted;
    }
});

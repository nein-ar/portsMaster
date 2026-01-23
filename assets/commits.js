document.addEventListener('DOMContentLoaded', () => {
    const container = document.getElementById('commit-log-container');
    if (!container) return;

    const baseUrl = document.body.dataset.baseUrl || '';
    const url = container.dataset.commitsUrl || (baseUrl + '/commits.json');
    const authorFilter = document.getElementById('author-filter');
    const timeframeFilter = document.getElementById('timeframe-filter');
    const srcUrl = document.body.dataset.sourceCodeUrl || 'https://codeberg.org/derivelinux/ports';

    let allCommits = [];

    function render(filtered) {
        if (!filtered || filtered.length === 0) {
            container.innerHTML = '<p style="padding:20px; text-align:center;">No matching commits found.</p>';
            return;
        }

        container.innerHTML = filtered.slice(0, 100).map(c => {
            const date = new Date(c.date);
            const dateStr = isNaN(date) ? '---' : date.toISOString().slice(0,16).replace('T',' ');
            const lines = (c.message || '').split('\n');
            const firstLine = lines[0] || '';
            const rest = lines.slice(1).join(' ').trim();
            const shortHash = (c.hash || '').slice(0,7);
            
            let filesHtml = '';
            if (c.added_files?.length) filesHtml += `<span class="files-added">+${c.added_files.length}</span>`;
            if (c.modified_files?.length) filesHtml += `<span class="files-modified">M${c.modified_files.length}</span>`;
            if (c.deleted_files?.length) filesHtml += `<span class="files-deleted">-${c.deleted_files.length}</span>`;

            return `
                <div class="commit-entry">
                    <span class="commit-time">${dateStr}</span>
                    <span class="commit-author">${c.author || 'unknown'}</span>
                    <a href="${srcUrl}/commit/${c.hash}" target="_blank" class="commit-link">${firstLine}</a>
                    ${rest ? `<br/><span class="commit-msg">${rest}</span>` : ''}
                    <div class="commit-file-stats">${filesHtml}</div>
                </div>
            `;
        }).join('');
    }

    function applyFilters() {
        const author = authorFilter?.value;
        const timeframe = timeframeFilter?.value;
        let filtered = allCommits;
        if (author) filtered = filtered.filter(c => c.author === author);
        if (timeframe && timeframe !== "All time") {
            const now = Date.now();
            const dayMs = 86400000;
            let limit = 0;
            if (timeframe === "Last 7 days") limit = now - 7 * dayMs;
            else if (timeframe === "Last 30 days") limit = now - 30 * dayMs;
            filtered = filtered.filter(c => new Date(c.date).getTime() > limit);
        }
        render(filtered);
    }

    fetch(url)
        .then(r => {
            if (!r.ok) throw new Error(`HTTP ${r.status}`);
            return r.json();
        })
        .then(data => {
            allCommits = (data || []).filter(c => c && c.hash);
            if (authorFilter) {
                const authors = [...new Set(allCommits.map(c => c.author))].filter(Boolean).sort();
                authorFilter.innerHTML = '<option value="">All Authors</option>' + 
                    authors.map(a => `<option value="${a}">${a}</option>`).join('');
                authorFilter.addEventListener('change', applyFilters);
            }
            timeframeFilter?.addEventListener('change', applyFilters);
            render(allCommits);
        })
        .catch(e => {
            container.innerHTML = `<p style="padding:20px; color:var(--warning-color); text-align:center;">Error: ${e.message}</p>`;
        });
});
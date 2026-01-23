document.addEventListener('DOMContentLoaded', () => {
    const container = document.getElementById('commit-log-container');
    if (!container) return;

    const baseUrl = document.body.dataset.baseUrl || '';
    const url = container.dataset.commitsUrl || (baseUrl + '/commits.json');
    const authorFilter = document.getElementById('author-filter');
    const timeframeFilter = document.getElementById('timeframe-filter');
    const categoryFilter = document.getElementById('category-filter');
    const srcUrl = document.body.dataset.sourceCodeUrl || 'https://codeberg.org/derivelinux/ports';

    let allCommits = [];

    function render(filtered) {
        if (!filtered || filtered.length === 0) {
            container.innerHTML = '<p class="p-20 text-center">No matching commits found.</p>';
            return;
        }

        container.innerHTML = filtered.slice(0, 100).map(c => {
            const date = new Date(c.date);
            const dateStr = isNaN(date) ? '---' : date.toISOString().slice(0,16).replace('T',' ') + ' UTC';
            const lines = (c.message || '').split('\n');
            const firstLine = lines[0] || '';
            const rest = lines.slice(1).join('<br/>').trim();
            const shortHash = (c.hash || '').slice(0,7);
            
            let filesHtml = '';
            const allFiles = [
                ...(c.added_files || []).map(f => `<span class="file-added">${f}</span>`),
                ...(c.modified_files || []).map(f => `<span class="file-modified">${f}</span>`),
                ...(c.deleted_files || []).map(f => `<span class="file-deleted">${f}</span>`)
            ];
            if (allFiles.length > 0) {
                filesHtml = `<div class="commit-files">${allFiles.join('')}</div>`;
            }

            return `
                <div class="commit-entry">
                    <div class="commit-header">
                        <span class="commit-hash">${shortHash}</span>
                        <span class="commit-time">${dateStr}</span>
                        <span class="commit-author">${c.author || 'unknown'}</span>
                    </div>
                    <div class="commit-title"><a href="${srcUrl}/commit/${c.hash}" target="_blank">${firstLine}</a></div>
                    ${rest ? `<div class="commit-msg">${rest}</div>` : ''}
                    ${filesHtml}
                </div>
            `;
        }).join('');
    }

    function applyFilters() {
        const author = authorFilter?.value;
        const timeframe = timeframeFilter?.value;
        const category = categoryFilter?.value;

        let filtered = allCommits;
        if (author) filtered = filtered.filter(c => c.author === author);
        
        if (category) {
            filtered = filtered.filter(c => {
                const files = [...(c.added_files || []), ...(c.modified_files || []), ...(c.deleted_files || [])];
                return files.some(f => f.startsWith(category + '/'));
            });
        }

        if (timeframe && timeframe !== "all time") {
            const now = Date.now();
            const dayMs = 86400000;
            let limit = 0;
            if (timeframe === "last 24 hours") limit = now - dayMs;
            else if (timeframe === "last 7 days") limit = now - 7 * dayMs;
            else if (timeframe === "last 30 days") limit = now - 30 * dayMs;
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
                authorFilter.innerHTML = '<option value="">all authors</option>' + 
                    authors.map(a => `<option value="${a}">${a}</option>`).join('');
                authorFilter.addEventListener('change', applyFilters);
            }
            if (categoryFilter) categoryFilter.addEventListener('change', applyFilters);
            timeframeFilter?.addEventListener('change', applyFilters);
            render(allCommits);
        })
        .catch(e => {
            container.innerHTML = `<p class="p-20 text-error text-center">Error: ${e.message}</p>`;
        });
});

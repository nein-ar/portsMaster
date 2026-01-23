document.addEventListener('DOMContentLoaded', () => {
    // Handle dynamic dimensions without inline style="" attribute in HTML source
    document.querySelectorAll('.js-width-bar').forEach(bar => {
        const width = bar.dataset.width;
        if (width) {
            bar.style.width = width;
        }
    });

    document.querySelectorAll('.js-height-bar').forEach(bar => {
        const height = bar.dataset.height;
        if (height) {
            bar.style.height = height;
        }
    });
});

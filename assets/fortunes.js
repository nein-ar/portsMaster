document.addEventListener("DOMContentLoaded", function() {
    const fortuneBox = document.getElementById('fortune-box');
    const fortuneText = document.getElementById('fortune-text');

    if (!fortuneBox || !fortuneText) {
        return;
    }

    fetch('/assets/fortunes.txt')
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            return response.text();
        })
        .then(text => {
            const fortunes = text.split('!---').map(f => f.trim()).filter(f => f.length > 0);
            if (fortunes && fortunes.length > 0) {
                const randomIndex = Math.floor(Math.random() * fortunes.length);
                fortuneText.textContent = fortunes[randomIndex];
            } else {
                fortuneBox.classList.add('display-none');
            }
        })
        .catch(e => {
            console.error("Error loading fortunes:", e);
            fortuneBox.classList.add('display-none');
        });
});
document.addEventListener("DOMContentLoaded", function() {
    const fortuneBox = document.getElementById('fortune-box');
    const fortuneText = document.getElementById('fortune-text');

    if (!fortuneBox || !fortuneText) {
        return;
    }

    // fortunes are embedded in the script via generator
    //
    if (typeof fortunes !== 'undefined' && fortunes.length > 0) {
        const randomIndex = Math.floor(Math.random() * fortunes.length);
        fortuneText.textContent = fortunes[randomIndex];
    } else {
        fortuneBox.classList.add('display-none');
    }
});

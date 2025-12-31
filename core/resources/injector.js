(function() {
    const debugButton = document.createElement('div');
    debugButton.style.position = 'fixed';
    debugButton.style.bottom = '20px';
    debugButton.style.right = '20px';
    debugButton.style.width = '50px';
    debugButton.style.height = '50px';
    debugButton.style.borderRadius = '50%';
    debugButton.style.backgroundColor = '#007bff';
    debugButton.style.color = 'white';
    debugButton.style.textAlign = 'center';
    debugButton.style.lineHeight = '50px';
    debugButton.style.cursor = 'pointer';
    debugButton.style.zIndex = '10000';
    debugButton.style.fontSize = '24px';
    debugButton.innerHTML = '\uD83D\uDC1E';
    debugButton.title = 'Open Debugger';

    debugButton.addEventListener('click', () => {
        window.open('__DEBUG_URL__', '_blank');
    });

    document.body.appendChild(debugButton);
})();

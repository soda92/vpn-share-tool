(function () {
  var checkInterval;

  function startPolling() {
    if (checkInterval) clearInterval(checkInterval);

    var attempts = 0;
    var maxAttempts = 60; // 30 seconds (60 * 500 ms)

    // Reset input on polling start (new image)
    var input = document.getElementById('verifyCode');
    if (input) input.value = '';

    checkInterval = setInterval(function () {
      attempts++;
      if (attempts > maxAttempts) {
        clearInterval(checkInterval);
        return;
      }

      fetch('/_proxy/captcha-solution')
        .then(function (res) {
          if (res.ok) return res.text();
          throw new Error('Not ready');
        })
        .then(function (code) {
          // Check if input is empty (to avoid overwriting user edits if they started typing?)
          // But if we just started polling, we cleared it.
          if (code && code.trim() !== "") {
            var input = document.getElementById('verifyCode');
            if (input) {
              input.value = code;
              console.log('Auto-filled Captcha: ' + code);

              var event = new Event('input', { bubbles: true });
              input.dispatchEvent(event);

              clearInterval(checkInterval);
            }
          }
        })
        .catch(function (e) { console.error('Captcha solution fetch error:', e); });
    }, 500); // Poll faster
  }

  // Start on load
  startPolling();

  // Restart on image click
  var img = document.getElementById('img');
  if (img) {
    img.addEventListener('click', function () {
      console.log('Captcha refreshed, restarting solver...');
      // Wait a bit for the new request to trigger
      setTimeout(startPolling, 500);
    });
  }
})();
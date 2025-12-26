/**
 * CAPTCHA handling for server-side CAPTCHA
 */

/**
 * Refresh the CAPTCHA by fetching a new CAPTCHA ID from the server and updating the image and hidden input.
 * @returns {void}
 */
function refreshCaptcha() {
    // Generate a new client-side CAPTCHA image and hash
    generateClientCaptcha();
}

// Utility: compute SHA-256 and return base64 string
function sha256Base64(text) {
    if (!window.crypto || !window.crypto.subtle) {
        // Fallback: use btoa of the text (not secure) if SubtleCrypto unavailable
        return Promise.resolve(btoa(unescape(encodeURIComponent(text))));
    }
    var enc = new TextEncoder();
    return window.crypto.subtle.digest('SHA-256', enc.encode(text)).then(function(hash) {
        var bytes = new Uint8Array(hash);
        var binary = '';
        for (var i = 0; i < bytes.byteLength; i++) {
            binary += String.fromCharCode(bytes[i]);
        }
        return btoa(binary);
    });
}

// Create and render a random 6-digit captcha in canvas and store its hash
function generateClientCaptcha() {
    var canvas = document.getElementById('captcha-canvas');
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    // adjust for device pixel ratio
    var ratio = window.devicePixelRatio || 1;
    var w = 300;
    var h = 80;
    canvas.width = w * ratio;
    canvas.height = h * ratio;
    canvas.style.width = '100%';
    canvas.style.height = 'auto';
    ctx.scale(ratio, ratio);

    // detect dark style (fallback false)
    var isDark = false;
    try {
        if (window.Helpers && typeof window.Helpers.isDarkStyle === 'function') isDark = !!window.Helpers.isDarkStyle();
    } catch (e) {
        isDark = false;
    }

    // choose color palette depending on theme
    var palette = {
        background: isDark ? '#1a1a1a' : '#f8fff8',
        dotBase: isDark ? 'rgba(255,255,255,' : 'rgba(34, 139, 34,',
        lineBase: isDark ? 'rgba(255,255,255,0.10)' : 'rgba(34, 139, 34, 0.25)',
        textEven: isDark ? '#aee0a6' : '#1b6f3a',
        textOdd: isDark ? '#8fd2a7' : '#2e8b57'
    };

    // background
    ctx.fillStyle = palette.background;
    ctx.fillRect(0, 0, w, h);

    // Allow configurable difficulty; default to medium
    window.CAPTCHA_OPTIONS = window.CAPTCHA_OPTIONS || {};
    var difficulty = window.CAPTCHA_OPTIONS.difficulty || 'medium'; // 'easy'|'medium'|'hard'

    // character set (avoid ambiguous chars like 0, O, 1, I, l)
    var charset = '23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';

    // set parameters per difficulty
    var length = 6, dotCount = 80, lineCount = 6, maxRotation = 10, fontMin = 32, fontMax = 40;
    if (difficulty === 'easy') {
        length = 5; dotCount = 60; lineCount = 4; maxRotation = 8; fontMin = 34; fontMax = 40;
    } else if (difficulty === 'hard') {
        length = 7; dotCount = 140; lineCount = 12; maxRotation = 22; fontMin = 26; fontMax = 46;
    } else {
        length = 6; dotCount = 100; lineCount = 8; maxRotation = 14; fontMin = 30; fontMax = 42;
    }

    // generate random characters
    var digits = '';
    for (var i = 0; i < length; i++) {
        digits += charset.charAt(Math.floor(Math.random() * charset.length));
    }

    // draw noise dots — more when difficulty high
    for (var i = 0; i < dotCount; i++) {
        ctx.fillStyle = palette.dotBase + (0.05 + Math.random()*0.75) + ')';
        ctx.beginPath();
        var radius = 0.8 + Math.random() * 2.5;
        ctx.arc(Math.random() * w, Math.random() * h, radius, 0, Math.PI*2);
        ctx.fill();
    }

    // draw lines and bezier curves to increase OCR difficulty
    ctx.lineWidth = 1 + Math.random() * 1.6;
    for (var i = 0; i < lineCount; i++) {
        ctx.strokeStyle = palette.lineBase;
        if (Math.random() > 0.45) {
            // bezier curve
            ctx.beginPath();
            var x1 = Math.random() * w, y1 = Math.random() * h;
            var cp1x = Math.random() * w, cp1y = Math.random() * h;
            var cp2x = Math.random() * w, cp2y = Math.random() * h;
            var x2 = Math.random() * w, y2 = Math.random() * h;
            ctx.moveTo(x1, y1);
            ctx.bezierCurveTo(cp1x, cp1y, cp2x, cp2y, x2, y2);
            ctx.stroke();
        } else {
            // straight or slightly curved line
            ctx.beginPath();
            ctx.moveTo(Math.random()*w, Math.random()*h);
            ctx.lineTo(Math.random()*w, Math.random()*h);
            ctx.stroke();
        }
    }

    // draw characters with random fonts/sizes/rotation/shadow — spaced evenly but with jitter
    var fonts = ['monospace','Courier New','Arial','Georgia','Times New Roman','Trebuchet MS','Verdana'];
    ctx.textBaseline = 'middle';
    var charX = 16 + Math.random()*6;
    for (var i = 0; i < digits.length; i++) {
        var ch = digits[i];
        ctx.save();
        var fontSize = Math.floor(fontMin + Math.random() * (fontMax - fontMin));
        var fontFace = fonts[Math.floor(Math.random()*fonts.length)];
        var fontWeight = Math.random() > 0.25 ? 'bold ' : '';
        ctx.font = fontWeight + fontSize + 'px ' + fontFace;

        var x = charX + i * (w - 32) / digits.length + (Math.random()*18 - 9);
        var y = h/2 + (Math.random()*18 - 9);
        var angle = (Math.random()*2*maxRotation - maxRotation) * Math.PI / 180;
        ctx.translate(x,y);
        ctx.rotate(angle);

        // small shadow
        ctx.shadowColor = 'rgba(0,0,0,' + (0.15 + Math.random()*0.25) + ')';
        ctx.shadowBlur = Math.random()*3;

        // alternate fill colors for contrast
        ctx.fillStyle = (i%2===0) ? palette.textEven : palette.textOdd;
        ctx.fillText(ch, 0, 0);

        // occasional faint stroke to increase complexity
        if (Math.random() > 0.6) {
            ctx.lineWidth = Math.max(0.6, Math.random()*1.6);
            ctx.strokeStyle = 'rgba(0,0,0,0.08)';
            ctx.strokeText(ch, 0, 0);
        }

        ctx.restore();
    }

    // store hash and clear client valid flag
    sha256Base64(digits).then(function(hash) {
        var hidden = document.getElementById('captcha_hash');
        if (hidden) hidden.value = hash;
        var validFlag = document.getElementById('client_captcha_valid');
        if (validFlag) validFlag.value = '0';
        // hide status
        var status = document.getElementById('captcha-status');
        if (status) { status.style.display = 'none'; status.className = 'captcha-status text-muted'; status.innerHTML='&nbsp;'; }
        // also clear input
        var input = document.getElementById('captcha-input');
        if (input) input.value = '';
    });
}

// initialize on load
document.addEventListener('DOMContentLoaded', function(){
    // generate captcha if canvas present
    setTimeout(function(){ generateClientCaptcha(); }, 50);
});

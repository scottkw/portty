package web

import "embed"

//go:embed dashboard.html terminal.html link-invalid.html desktop-off.html desktop-link-invalid.html user-refused.html service-retry.html desktop-guest-refused.html desktop-guest-link-invalid.html
//go:embed assets/terminal.js assets/terminal.css
//go:embed assets/dashboard.css
//go:embed vendor/xterm/xterm.js vendor/xterm/xterm.css vendor/xterm/addon-fit.js vendor/xterm/VERSION
//go:embed vendor/xterm/addons/addon-webgl.js vendor/xterm/addons/addon-unicode11.js vendor/xterm/addons/addon-clipboard.js vendor/xterm/addons/addon-search.js vendor/xterm/addons/addon-web-links.js
//go:embed vendor/xterm/addons/addon-image.js vendor/xterm/addons/addon-serialize.js vendor/xterm/addons/addon-progress.js
var WebFS embed.FS

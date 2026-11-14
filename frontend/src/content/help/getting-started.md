## Getting Started

AgentHub runs shell sessions in terminal tabs. Sessions persist across app restarts via a background daemon, so you can close and reopen the window without losing your work.

### Creating a Session

Click the **+ New Session** button in the Hub toolbar to open the session creation modal. Pick a working directory and the session starts a shell there, in a new terminal tab.

### Running Shell Sessions

Every session is a plain shell session. Shell sessions auto-close when the shell exits naturally.

### Switching Between Sessions

Open sessions appear as tabs along the top of the window. Click any tab to switch to that session. The **Hub** view (Home icon in the sidebar) shows all sessions as cards grouped by working directory — click a card to open or expand that session.

### Sharing Sessions Over the Network

AgentHub serves sessions over your Tailscale network with automatic HTTPS. In the Hub, click the **Share** button on any session card. Toggle "Share the session" to generate read-only and read/write links.

### Settings Overview

Open **Settings** from the sidebar (or press the gear icon). Settings is divided into sections: Plugins (feature toggles), Web Server (URL, QR code, copy link), Paths (CLI and shell binary locations), and Appearance (terminal theme, font size). Changes take effect immediately or after saving, depending on the setting.

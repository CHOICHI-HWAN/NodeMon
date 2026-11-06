// SPDX-License-Identifier: GPL-3.0-or-later
// Which connector file belongs to which computer, and the commands to run it.
// File names match what build-connector.sh writes into dist/; the ZIP files of a GitHub Release hold the same programs.

export type Os = "linux" | "darwin" | "windows";
export type Arch = "amd64" | "arm64" | "arm";
export type Build = "download" | "dist" | "local"; // download the release ZIP, copy a file from dist/, or build on this very computer

export const OS_LABEL: Record<Os, string> = { linux: "Linux", darwin: "macOS", windows: "Windows" };

export const ARCHES: Record<Os, { arch: Arch; label: string; reported: string }[]> = {
  linux: [
    { arch: "amd64", label: "x86_64 — 일반 PC·서버·대부분의 NAS", reported: "x86_64" },
    { arch: "arm64", label: "aarch64 — 라즈베리파이 4·5(64비트 OS), ARM 서버", reported: "aarch64" },
    { arch: "arm", label: "armv7l — 라즈베리파이 32비트 OS", reported: "armv7l" },
  ],
  darwin: [
    { arch: "arm64", label: "Apple 칩 (M1·M2·M3·M4)", reported: "arm64" },
    { arch: "amd64", label: "Intel 맥", reported: "x86_64" },
  ],
  windows: [{ arch: "amd64", label: "64비트 Windows (대부분)", reported: "AMD64" }],
};

export function guessOs(): Os {
  const p = `${navigator.platform} ${navigator.userAgent}`;
  return /Win/i.test(p) ? "windows" : /Mac/i.test(p) ? "darwin" : "linux";
}

export function fileName(os: Os, arch: Arch, build: Build): string {
  const exe = os === "windows" ? ".exe" : "";
  return build === "local" ? `nodemon-connector${exe}` : `nodemon-connector-${os}-${arch}${exe}`;
}

export const checkCommand = (os: Os) => (os === "windows" ? "$env:PROCESSOR_ARCHITECTURE" : "uname -m");

export function runCommand(os: Os, file: string, args: string): string {
  return os === "windows" ? `.\\${file} ${args}` : `chmod +x ./${file} && ./${file} ${args}`;
}

export const REPO_URL = "https://github.com/CHOICHI-HWAN/NodeMon";
export const RELEASES_URL = `${REPO_URL}/releases`;

// "latest" always points to the newest release, so these links never need to change.
export const zipName = (os: Os, arch: Arch) => `nodemon-connector-${os}-${arch}.zip`;
export const downloadUrl = (name: string) => `${RELEASES_URL}/latest/download/${name}`;

// Fetching from the terminal of the target computer is the simplest path; curl also avoids macOS's download quarantine.
// The ZIP holds the program under the name `file`, so the commands that follow work unchanged.
export function downloadCommand(os: Os, arch: Arch): string {
  const zip = zipName(os, arch);
  const url = downloadUrl(zip);
  return os === "windows"
    ? `Invoke-WebRequest ${url} -OutFile ${zip}; Expand-Archive ${zip} -DestinationPath . -Force`
    : `curl -fLO ${url} && unzip -o ${zip}`;
}

export const BUILD_LOCAL = "cd connector && go build -o nodemon-connector . && cd ..";

// The service definitions are built from lines so no heredoc terminator ever has to sit inside this source.
const lines = (...l: string[]) => l.join("\n");

// Commands that keep the connector running after a reboot. They reuse the credentials the first run saved.
export function autostartScript(os: Os, file: string): string {
  if (os === "linux") {
    return lines(
      `sudo install -m 755 ./${file} /usr/local/bin/nodemon-connector`,
      "sudo mkdir -p /etc/nodemon",
      "sudo cp ~/.config/nodemon/connector.json /etc/nodemon/connector.json",
      "sudo tee /etc/systemd/system/nodemon-connector.service >/dev/null <<'EOF'",
      "[Unit]",
      "Description=NodeMon connector",
      "After=network-online.target netdata.service",
      "Wants=network-online.target",
      "",
      "[Service]",
      "ExecStart=/usr/local/bin/nodemon-connector --config /etc/nodemon/connector.json",
      "Restart=always",
      "RestartSec=10",
      "",
      "[Install]",
      "WantedBy=multi-user.target",
      "EOF",
      "sudo systemctl daemon-reload",
      "sudo systemctl enable --now nodemon-connector",
      "systemctl status nodemon-connector --no-pager",
    );
  }
  if (os === "darwin") {
    return lines(
      `mkdir -p ~/nodemon && cp ./${file} ~/nodemon/nodemon-connector`,
      "cat > ~/Library/LaunchAgents/com.nodemon.connector.plist <<EOF",
      '<?xml version="1.0" encoding="UTF-8"?>',
      '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">',
      '<plist version="1.0"><dict>',
      "  <key>Label</key><string>com.nodemon.connector</string>",
      "  <key>ProgramArguments</key><array><string>$HOME/nodemon/nodemon-connector</string></array>",
      "  <key>RunAtLoad</key><true/>",
      "  <key>KeepAlive</key><true/>",
      "</dict></plist>",
      "EOF",
      "launchctl load -w ~/Library/LaunchAgents/com.nodemon.connector.plist",
    );
  }
  return lines(
    "mkdir C:\\nodemon",
    `copy .\\${file} C:\\nodemon\\nodemon-connector.exe`,
    'schtasks /Create /TN "NodeMon Connector" /TR "C:\\nodemon\\nodemon-connector.exe" /SC ONLOGON /F',
    'schtasks /Run /TN "NodeMon Connector"',
  );
}

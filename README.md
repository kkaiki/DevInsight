# DevInsights for Visual Studio Code

[DevInsights] is a tool that automatically tracks the time you spend coding in Visual Studio Code and provides detailed insights into your productivity. It also integrates with Discord to rank total coding time, allowing you to see which users have worked the most. Additionally, DevInsights can be used with multiple IDEs and editors, enabling a comprehensive view of your coding activity across different platforms.

Installation
1.Press F1 or ⌘ + Shift + P and type install. Pick Extensions: Install Extension.

2.Type devinsights and hit enter.

3.Enter your Discord ID, then press enter.
(If you’re not prompted, press F1 or ⌘ + Shift + P then type DevInsights Discord ID.)
   How to Copy Your Discord ID:
      Open Discord and go to User Settings.
      Under Advanced, enable Developer Mode.
      Right-click your username in the user list or your profile, and select Copy ID.
4.Use VSCode and your coding activity will be ranked weekly.

## Usage

Each week, a Discord bot ranks users based on the amount of time they have spent coding, providing a leaderboard of coding activity.

## Configuring

VS Code Configuration:
Press ⌘ + Shift + P and type DevInsights: Input Discord Id to set your Discord ID.

## Troubleshooting

Checking Error Logs:
Errors outside the scope of DevInsights are recorded in the $HOME/.devinsights/devinsights.log file.

Information for Windows Users:
If you are using DevInsights behind a corporate proxy, it is recommended to enable your Windows Root Certs inside VS Code by installing the [win-ca] extension. Press Ctrl + Shift + X, search for win-ca, and install it.

### SSH configuration

If you're connected to a remote host using the [ssh extension](https://code.visualstudio.com/docs/remote/ssh) you might want to force DevInshghts to run locally instead on the server. This configuration is needed when the server you connect is shared among other people. Please follow [this](https://code.visualstudio.com/docs/remote/ssh#_advanced-forcing-an-extension-to-run-locally-remotely) guide.

## Uninstalling

1.Click the Extensions sidebar item in VS Code.

2.Type devinsights and hit enter.

3.Click the settings icon next to DevInsights, then click Uninstall.

4.Delete the ~/.devinsights* files in your home directory, unless you’re still using DevInsights with another IDE.
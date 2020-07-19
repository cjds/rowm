# rowm

rowm is a hybrid tiling/floating window manager, aiming to replicate the ergonamics of `terminator` terminal emulator, but applied to all windows.

A short video of what it looks like [here](https://i.imgur.com/Bk5N5MY.mp4).

# Installation
While I use rowm in my day to day, there is no guarantee of stability so use at your own risk!

To install, you have two options:
- If you have `docker` installed, you can run `sudo ./docker-install.sh`.
- If you have `go` and `go get` installed, you can run `sudo ./go-install.sh`.

After installing, restart your computer (logging out may not be enough), and the option to use rowm as your window manager should show up as a setting in your login screen.

It's generally not wise to run random shell scripts with sudo but these should be pretty easy to inspect.

# Default Configuration
All key combinations described here are easily editable in `config.go`. `Mod4` is used a lot, which is commonly assigned to the Windows key.

#### Logging out
Press `Mod4-Backspace`

#### Builtin Commands
Some commands have builtin keyboard shortcuts, namely:

`Mod4-t: x-terminal-emulator`

`Mod4-w: google-chrome`

`Mod4-p: XDG_CURRENT_DESKTOP=GNOME gnome-control-center`

`Mod4-o: xdg-open .`

More commands can be added to the builtin commands in `config.go`.

`Mod4-f` brings up a dialog to run an arbitrary command.

#### Window controls
`Mod4-up/left/right/down` will move the window to anchor points around the screen, as well as keep moving them across other screens if they are available.

`Mod4-d` will close a frame.

`Mod4-x` will toogle a frame as expanded, making it the same size as its container.

`Mod4-h` will hide the container decorations.

`Mod4-q` will pop out a frame into its own container.

`Alt-Tab`/`Alt-Shift-Tab` works like you'd expect.

`Mod-Shift-[0-9]` assigns a goto hotkey to the selected frame, so that when you press the equivalent `Mod4-[0-9]` it will minimize/unminimize that window.

`Mod4-Tab` and `Mod4-asciitilde` will cycle the focus in the frames inside of a window.

If an internal video is being fullscreened, sometimes you may need to resize or move the window a little to have the internal video fill the screen.

#### Taskbar
The taskbar can be toggled with `Mod4-s`. `Mod4-Shift-down` will minimize the focused container to the taskbar. `Mod4-Shift-left/right` will scroll the taskbar if lots of windows are open.

The display time format can be changed in `config.go`, but things may be a bit funky if the time format does not have constant size.

#### Background image
Any photo named `$USER/.config/rowm/bg.png` will be used as a background image. You can change the path in `config.go`.

#### Splitting
To subdivide a window, press `Mod4-e` for a horizontal split or `Mod4-r` for a vertical split. A command window will pop up to take in a command to launch, but you can use the keyboard shortcuts to launch a builtin command, bypassing the command prompt.

To split for existing frames or containers:

`Mod4-c` selects a frame for yanking.

`Mod4-Shift-c` selects a container for yanking.

`Mod4-v` on a different frame will add the selection as a horizontal child.
`Mod4-b` on a different frame will add the selection as a vertical child.

#### Volume
Can be controlled with mute button to mute, and volume up/down to raise/lower volume. If you do not have these buttons you can change the mapping in `config.go`

#### Brightness
If brightness controls are not working, check what folder exists in `/sys/class/backlight/` and change the Backlight value in `config.go` to match.

#### Switching the scroll direction
Edit the following file `/usr/share/X11/xorg.conf.d/40-libinput.conf`. Find the section with the `touchpad` identifier and add the following line.

```
Section "InputClass"
        Identifier "libinput touchpad catchall"
        ...
        Option "NaturalScrolling" "True"
EndSection
```
You can also switch it for mice by changing the `pointer` catchall.

## Monitor hotplugging
If you are planning on hotplugging monitors, it is recommended you install arandr and autorandr.

`arandr` is a graphical utility to help you configure your screens. Once you have them configured how you like, use `autorandr --save <profile>` to save the configuration and `autorandr --default <profile>` to make it default. Once this is done the monitor setup should stay sticky through disconnects and reconnects.





## Logging
To tail the logs run `journalctl -f /usr/lib/gdm3/gdm-x-session`.

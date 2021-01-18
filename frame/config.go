package frame

import (
	"fmt"
	"github.com/BurntSushi/xgbutil/xcursor"
	"github.com/spf13/viper"
	"log"
	"os/user"
	"path"
	"time"
)

type StringWithHelp struct {
	Data	string	`mapstructure:"data"`
	Help	string	`mapstructure:"help"`
}

type Config struct {
	Lock                      string `mapstructure:"lock"`
	Shell                     string `mapstructure:"shell"`
	TabByFrame                bool `mapstructure:"tab_frame"`
	TabForward                StringWithHelp `mapstructure:"tab_forward"`
	TabBackward               StringWithHelp `mapstructure:"tab_backward"`
	ButtonDrag                string `mapstructure:"button_drag"`
	ButtonClick               string `mapstructure:"button_click"`
	ButtonRightClick          string `mapstructure:"button_right_click"`
	SplitVertical             StringWithHelp `mapstructure:"split_vertical"`
	SplitHorizontal           StringWithHelp `mapstructure:"split_horizontal"`
	RunCmd                    StringWithHelp `mapstructure:"run_cmd"`
	Shutdown                  string `mapstructure:"shutdown"`
	CloseFrame                StringWithHelp `mapstructure:"close_frame"`
	ToggleExpandFrame         StringWithHelp `mapstructure:"toggle_expand_frame"`
	ToggleExternalDecorator   string `mapstructure:"toggle_external_decorator"`
	ToggleTaskbar             string `mapstructure:"toggle_taskbar"`
	PopFrame                  StringWithHelp `mapstructure:"pop_frame"`
	ResetSize                 string `mapstructure:"reset_size"`
	Minimize                  string `mapstructure:"minimize"`
	WindowUp		  StringWithHelp `mapstructure:"window_up"`
	WindowDown                StringWithHelp `mapstructure:"window_down"`
	WindowLeft                StringWithHelp `mapstructure:"window_left"`
	WindowRight               StringWithHelp `mapstructure:"window_right"`
	VolumeUp                  string `mapstructure:"volume_up"`
	VolumeDown                string `mapstructure:"volume_down"`
	BrightnessUp              string `mapstructure:"brightness_up"`
	BrightnessDown            string `mapstructure:"brightness_down"`
	Backlight                 string `mapstructure:"backlight"`
	VolumeMute                string `mapstructure:"volume_mute"`
	FocusNext                 StringWithHelp `mapstructure:"focus_next"`
	FocusPrev                 StringWithHelp `mapstructure:"focus_prev"`
	ElemSize                  int `mapstructure:"elem_size"`
	CloseCursor               int `mapstructure:"close_cursor"`
	DefaultShapeRatio         Rectf
	SeparatorColor            uint32 `mapstructure:"seperator_color"`
	GrabColor                 uint32 `mapstructure:"grab_color"`
	FocusColor                uint32 `mapstructure:"focus_color"`
	CloseColor                uint32 `mapstructure:"close_clor"`
	MaximizeColor             uint32 `mapstructure:"maximize_color"`
	MinimizeColor             uint32 `mapstructure:"minimize_color"`
	ResizeColor               uint32 `mapstructure:"resize_color"`
	TaskbarBaseColor          uint32 `mapstructure:"taskbar_base_color"`
	TaskbarTextColor          uint32 `mapstructure:"taskbar_text_color"`
	InternalPadding           int `mapstructure:"internal_padding"`
	BackgroundImagePath       string `mapstructure:"background_image_path"`
	BuiltinCommands           map[StringWithHelp]string `mapstructure:"builtin_commands"`
	FocusMarkerTime           time.Duration
	DoubleClickTime           time.Duration
	TaskbarHeight             int `mapstructure:"taskbar_height"`
	TaskbarSlideWidth         int `mapstructure:"taskbar_slide_width"`
	TaskbarSlideActiveColor   uint32 `mapstructure:"taskbar_slide_active_color"`
	TaskbarSlideInactiveColor uint32 `mapstructure:"taskbar_slide_inactive_color"`
	TaskbarFontSize           float64 `mapstructure:"taskbar_font_size"`
	TaskbarTimeBaseColor      uint32 `mapstructure:"taskbar_time_base_color"`
	TaskbarXPad               int `mapstructure:"taskbar_x_pad"`
	TaskbarYPad               int `mapstructure:"taskbar_y_pad"`
	TaskbarTimeFormat         string `mapstructure:"taskbar_time_format"`
	TaskbarBatFormat          string `mapstructure:"taskbar_bat_format"`
	TaskbarElementShape       Rect
	TaskbarMinMaxHeight       int `mapstructure:"taskbar_min_max_height"`
	TaskbarMinMaxColor        uint32 `mapstructure:"taskbar_min_max_color"`
	TaskbarSlideLeft          string `mapstructure:"taskbar_slide_left"`
	TaskbarSlideRight         string `mapstructure:"taskbar_slide_right"`
	CutSelectFrame            string `mapstructure:"cut_select_frame"`
	CutSelectContainer        string `mapstructure:"cut_select_container"`
	CopySelectHorizontal      StringWithHelp `mapstructure:"copy_select_horizontal"`
	CopySelectVertical        StringWithHelp `mapstructure:"copy_select_vertical"`
	SuspendCommand            string `mapstructure:"suspend_command"`
	BatteryWarningLevels      []int
	BatteryWarningDuration    time.Duration
	LaunchHelp                string `mapstructure:"launch_help"`
	GotoKeys                  map[string]string
}

func HomeDir() string {
	usr, err := user.Current()
	if err != nil {
		log.Println(err)
		return ""
	}
	return usr.HomeDir
}

func (c *Config) MinShape() Rect {
	return Rect{
		X: 0,
		Y: 0,
		W: 5 * c.ElemSize,
		H: 5 * c.ElemSize,
	}
}

func LoadConfig() Config {
	conf := DefaultConfig()
	viper.SetConfigName("rowm") // name of config file (without extension)
	viper.SetConfigType("json") // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/rowm/")
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("Didn't load config file ", viper.ConfigFileUsed(), ": ", err)
	}

	err = viper.Unmarshal(&conf)
	if err != nil {
		log.Println("Unable to decode into config struct ", viper.ConfigFileUsed(), " :", err)
	}
	return conf
}


// DefaultConfig reference for key strings
// https://github.com/BurntSushi/xgbutil/blob/master/keybind/keysymdef.go
func DefaultConfig() Config {
	return Config{
		Shell:                   "/bin/bash",
		Lock:                    "Mod4-l",
		TabByFrame:              true,
		TabForward:              StringWithHelp{Data: "Mod1-tab", Help: "Tab Forward"},
		TabBackward:             StringWithHelp{Data: "Mod1-Shift-tab", Help: "Tab Backward"},
		ButtonDrag:              "1",
		ButtonClick:             "1",
		ButtonRightClick:        "3",
		SplitVertical:           StringWithHelp{Data: "Mod4-r", Help: "Split Vertically"},
		SplitHorizontal:         StringWithHelp{Data: "Mod4-e", Help: "Split Horizontally"},
		RunCmd:                  StringWithHelp{Data: "Mod4-f", Help: "Run Command"},
		Shutdown:                "Mod4-BackSpace",
		CloseFrame:              StringWithHelp{Data: "Mod4-d", Help: "Close Frame"},
		ToggleExpandFrame:       StringWithHelp{Data: "Mod4-x", Help: "Toggle Expanded Frame"},
		ToggleExternalDecorator: "Mod4-h",
		ToggleTaskbar:           "Mod4-s",
		WindowUp:                StringWithHelp{Data: "Mod4-up", Help: "Window up"},
		WindowDown:              StringWithHelp{Data: "Mod4-down", Help: "Window Down"},
		WindowLeft:              StringWithHelp{Data: "Mod4-left", Help: "Window Left"},
		WindowRight:             StringWithHelp{Data: "Mod4-right", Help: "Window Right"},
		PopFrame:                StringWithHelp{Data: "Mod4-q", Help: "Pop Frame"},
		ResetSize:               "Mod4-Shift-up",
		Minimize:                "Mod4-Shift-down",
		VolumeUp:                "XF86AudioRaiseVolume",
		VolumeDown:              "XF86AudioLowerVolume",
		VolumeMute:              "XF86AudioMute",
		BrightnessUp:            "XF86MonBrightnessUp",
		BrightnessDown:          "XF86MonBrightnessDown",
		FocusNext:               StringWithHelp{Data: "Mod4-Tab", Help:"Focus Next"},
		FocusPrev:               StringWithHelp{Data: "Mod4-asciitilde", Help:"Focus Previous"},
		Backlight:               "intel_backlight",
		ElemSize:                10,
		CloseCursor:             xcursor.Dot,
		DefaultShapeRatio: Rectf{
			X: .1,
			Y: .1,
			W: .8,
			H: .8,
		},
		SeparatorColor:      0x777777,
		GrabColor:           0x339999,
		FocusColor:          0x9932cc,
		ResizeColor:         0x777777,
		TaskbarBaseColor:    0x222222,
		TaskbarTextColor:    0xbbbbbb,
		CloseColor:          0xff0000,
		MaximizeColor:       0x00ff00,
		MinimizeColor:       0xfdfd96,
		InternalPadding:     0,
		BackgroundImagePath: path.Join(HomeDir(), ".config/rowm/bg.png"),
		FocusMarkerTime:     time.Millisecond * 350,
		DoubleClickTime:     time.Millisecond * 500,
		BuiltinCommands: map[StringWithHelp]string{
			StringWithHelp{Data: "Mod4-t", Help:"Terminal"}:  "x-terminal-emulator",
			StringWithHelp{Data: "Mod4-w", Help:"Chrome"}:  "google-chrome",
			StringWithHelp{Data: "Mod4-p", Help: "Gnome"}:  "XDG_CURRENT_DESKTOP=GNOME gnome-control-center",
			StringWithHelp{Data: "Mod4-o", Help:" XDG"}:"xdg-open .",
			StringWithHelp{Data: "XF86AudioPlay", Help:"Pause"}: "dbus-send --print-reply --dest=org.mpris.MediaPlayer2.spotify /org/mpris/MediaPlayer2 org.mpris.MediaPlayer2.Player.PlayPause",
			StringWithHelp{Data: "XF86AudioPrev", Help: "Previous"}: "dbus-send --print-reply --dest=org.mpris.MediaPlayer2.spotify /org/mpris/MediaPlayer2 org.mpris.MediaPlayer2.Player.Previous",
			StringWithHelp{Data: "XF86AudioNext", Help: "Next"}: "dbus-send --print-reply --dest=org.mpris.MediaPlayer2.spotify /org/mpris/MediaPlayer2 org.mpris.MediaPlayer2.Player.Next",
			StringWithHelp{Data: "Print", Help: "Screenshot"}:"gnome-screenshot -i",
		},
		TaskbarHeight:        20,
		TaskbarFontSize:      12,
		TaskbarTimeBaseColor: 0x222222,
		TaskbarXPad:          5,
		TaskbarYPad:          5,
		TaskbarTimeFormat:    "2006 Mon Jan 02 - 15:04:05 (MST)",
		TaskbarBatFormat:     "%s%3d%%",
		TaskbarElementShape: Rect{
			X: 2,
			Y: 0,
			W: 16,
			H: 16,
		},
		TaskbarSlideWidth:         10,
		TaskbarSlideActiveColor:   0x666666,
		TaskbarSlideInactiveColor: 0x333333,
		TaskbarMinMaxHeight:       4,
		TaskbarMinMaxColor:        0x999999,
		TaskbarSlideLeft:          "Mod4-Shift-left",
		TaskbarSlideRight:         "Mod4-Shift-right",
		CutSelectFrame:            "Mod4-c",
		CutSelectContainer:        "Mod4-Shift-c",
		CopySelectHorizontal:      StringWithHelp{Data: "Mod4-v", Help: "Paste Horizontally"},
		CopySelectVertical:        StringWithHelp{Data: "Mod4-b", Help: "Paste Vertically"},
		SuspendCommand:            "systemctl suspend",
		BatteryWarningLevels:      []int{20, 10, 5, 1},
		BatteryWarningDuration:    time.Second * 2,
		LaunchHelp:                "Mod4-Shift-h",
		GotoKeys: map[string]string{
			"Mod4-Shift-0": "Mod4-0",
			"Mod4-Shift-1": "Mod4-1",
			"Mod4-Shift-2": "Mod4-2",
			"Mod4-Shift-3": "Mod4-3",
			"Mod4-Shift-4": "Mod4-4",
			"Mod4-Shift-5": "Mod4-5",
			"Mod4-Shift-6": "Mod4-6",
			"Mod4-Shift-7": "Mod4-7",
			"Mod4-Shift-8": "Mod4-8",
			"Mod4-Shift-9": "Mod4-9",
		},
	}
}

func (c *Config) BrightFile() string {
	return fmt.Sprintf("/sys/class/backlight/%s/brightness", c.Backlight)
}

func (c *Config) MaxBrightFile() string {
	return fmt.Sprintf("/sys/class/backlight/%s/max_brightness", c.Backlight)
}

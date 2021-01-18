package frame

import (
	"bytes"
	"image/color"

	"github.com/BurntSushi/freetype-go/freetype/truetype"

	"github.com/BurntSushi/xgb/xproto"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xrect"
	"github.com/BurntSushi/xgbutil/xwindow"

	"github.com/BurntSushi/wingo/misc"
	"github.com/BurntSushi/wingo/render"
	"github.com/BurntSushi/wingo/text"
	"github.com/BurntSushi/wingo/focus"
)

func ignoreFocus(modeByte, detailByte byte) bool {
	mode, detail := focus.Modes[modeByte], focus.Details[detailByte]

	if mode == "NotifyGrab" || mode == "NotifyUngrab" {
		return true
	}
	if detail == "NotifyAncestor" ||
		detail == "NotifyInferior" ||
		detail == "NotifyNonlinear" ||
		detail == "NotifyPointer" ||
		detail == "NotifyPointerRoot" ||
		detail == "NotifyNone" {

		return true
	}
	return false
}

type InputWithSuggestion struct {
	X      *xgbutil.XUtil
	theme  *InputWithSuggestionTheme
	config InputWithSuggestionConfig

	history      []string
	historyIndex int

	showing  bool
	do       func(inp *InputWithSuggestion, text string)
	canceled func(inp *InputWithSuggestion)

	win                          *xwindow.Window
	label                        *xwindow.Window
	input                        *text.Input
	suggestion                        *xwindow.Window
	bInp, bTop, bMid, bBot, bLft, bRht *xwindow.Window
	suggest       func(inp string) []string
	suggestionIndex int
}

func NewInputWithSuggestion(X *xgbutil.XUtil, theme *InputWithSuggestionTheme, config InputWithSuggestionConfig) *InputWithSuggestion {
	input := &InputWithSuggestion{
		X:       X,
		theme:   theme,
		config:  config,
		showing: false,
		do:      nil,
		history: make([]string, 0, 100),
		suggestionIndex: 0,
	}

	// Create all windows used for the base of the input prompt.
	cwin := func(p xproto.Window) *xwindow.Window {
		return xwindow.Must(xwindow.Create(X, p))
	}
	input.win = cwin(X.RootWin())
	input.label = cwin(input.win.Id)
	input.suggestion = cwin(input.win.Id)
	input.input = text.NewInput(X, input.win.Id, 1000, theme.Padding,
		theme.Font, theme.FontSize, theme.FontColor, theme.InvisibleBgColor)
	input.bInp = cwin(input.win.Id)
	input.bTop, input.bMid, input.bBot = cwin(input.win.Id), cwin(input.win.Id), cwin(input.win.Id)
	input.bLft, input.bRht = cwin(input.win.Id), cwin(input.win.Id)

	// Make the top-level window override redirect so the window manager
	// doesn't mess with us.
	input.win.Change(xproto.CwOverrideRedirect, 1)
	input.win.Listen(xproto.EventMaskFocusChange)
	input.input.Listen(xproto.EventMaskKeyPress)

	// Colorize the windows.
	cclr := func(w *xwindow.Window, clr render.Color) {
		w.Change(xproto.CwBackPixel, clr.Uint32())
	}
	cclr(input.win, input.theme.BgColor)
	cclr(input.bInp, input.theme.BorderColor)
	cclr(input.bTop, input.theme.BorderColor)
	cclr(input.bMid, input.theme.BorderColor)
	cclr(input.bBot, input.theme.BorderColor)
	cclr(input.bLft, input.theme.BorderColor)
	cclr(input.bRht, input.theme.BorderColor)

	// Map the sub-windows once.
	input.label.Map()
	input.input.Map()
	input.bInp.Map()
	input.bTop.Map()
	input.bMid.Map()
	input.bBot.Map()
	input.bLft.Map()
	input.bRht.Map()
	input.suggestion.Map()

	// Connect the key response handler.
	// The handler is responsible for tab completion and quitting if the
	// cancel key has been pressed.
	input.keyResponse().Connect(X, input.input.Id)
	input.focusResponse().Connect(X, input.win.Id)

	return input
}

func (inp *InputWithSuggestion) Showing() bool {
	return inp.showing
}

func (inp *InputWithSuggestion) Destroy() {
	inp.input.Destroy()
	inp.suggestion.Destroy()
	inp.label.Destroy()
	inp.bInp.Destroy()
	inp.bTop.Destroy()
	inp.bMid.Destroy()
	inp.bBot.Destroy()
	inp.bLft.Destroy()
	inp.bRht.Destroy()
	inp.win.Destroy()
}

func (inp *InputWithSuggestion) Id() xproto.Window {
	return inp.win.Id
}

func (inp *InputWithSuggestion) Show(workarea xrect.Rect, label string,
	do func(inp *InputWithSuggestion, text string), canceled func(inp *InputWithSuggestion), suggest func(inp string)[]string) bool {

	if inp.showing {
		return false
	}

	inp.win.Stack(xproto.StackModeAbove)
	inp.input.Reset()

	text.DrawText(inp.label, inp.theme.Font, inp.theme.FontSize,
		inp.theme.FontColor, inp.theme.BgColor, label)
	text.DrawText(inp.suggestion, inp.theme.Font, inp.theme.FontSize,
		inp.theme.FontColor, inp.theme.BgColor, " ")

	pad, bs := inp.theme.Padding, inp.theme.BorderSize
	width := (pad * 2) + (bs * 2) +
		inp.label.Geom.Width() + inp.theme.InputWidth
	height := (pad * 4) + (bs * 3) + inp.label.Geom.Height() + inp.suggestion.Geom.Height()

	// position the damn window based on its width/height (i.e., center it)
	posx := workarea.X() + workarea.Width()/2 - width/2
	posy := workarea.Y() + workarea.Height()/2 - height/2

	inp.win.MoveResize(posx, posy, width, height)
	inp.label.Move(bs+pad, pad+bs)
	inp.bInp.MoveResize(pad+inp.label.Geom.X()+inp.label.Geom.Width(), 0,
		bs, pad*2 + bs + inp.label.Geom.Height())
	inp.bTop.Resize(width, bs)
	inp.bMid.MoveResize(0, (pad*2) +  inp.label.Geom.Height() + bs, width, bs)
	inp.bBot.MoveResize(0, height-bs, width, bs)
	inp.bLft.Resize(bs, height)
	inp.bRht.MoveResize(width - bs, 0, bs, height)
	inp.input.Move(inp.bInp.Geom.X()+inp.bInp.Geom.Width(), bs)
	inp.suggestion.Move(pad,inp.label.Geom.Height() + pad*3 + bs*2)

	inp.showing = true
	inp.do = do
	inp.suggest = suggest
	inp.canceled = canceled
	inp.win.Map()
	inp.input.Focus()
	inp.historyIndex = len(inp.history)

	return true
}

func (inp *InputWithSuggestion) Hide() {
	if !inp.showing {
		return
	}

	inp.win.Unmap()
	inp.input.Reset()

	inp.showing = false
	inp.do = nil
	inp.canceled = nil
}

func (inp *InputWithSuggestion) focusResponse() xevent.FocusOutFun {
	f := func(X *xgbutil.XUtil, ev xevent.FocusOutEvent) {
		if !ignoreFocus(ev.Mode, ev.Detail) {
			if inp.canceled != nil {
				inp.canceled(inp)
			}
			inp.Hide()
		}
	}
	return xevent.FocusOutFun(f)
}

func (inp *InputWithSuggestion) keyResponse() xevent.KeyPressFun {
	f := func(X *xgbutil.XUtil, ev xevent.KeyPressEvent) {
		suggestionSlice := inp.suggest(string(inp.input.Text))
		if !inp.showing {
			return
		}

		mods, kc := keybind.DeduceKeyInfo(ev.State, ev.Detail)
		switch {
		case keybind.KeyMatch(X, "Up", mods, kc):
			if inp.historyIndex > 0 {
				inp.historyIndex--
				inp.input.SetString(inp.history[inp.historyIndex])
			}
		case keybind.KeyMatch(X, "Down", mods, kc):
			if inp.historyIndex < len(inp.history) {
				inp.historyIndex++
				if inp.historyIndex < len(inp.history) {
					inp.input.SetString(inp.history[inp.historyIndex])
				} else {
					inp.input.Reset()
				}
			}
		case keybind.KeyMatch(X, inp.config.BackspaceKey, mods, kc):
			inp.input.Remove()
		case keybind.KeyMatch(X, inp.config.ConfirmKey, mods, kc):
			if inp.do != nil {
				s := string(inp.input.Text)
				histLen := len(inp.history)

				// Don't added repeated entries.
				if histLen == 0 || s != inp.history[histLen-1] {
					inp.history = append(inp.history, s)
				}
				inp.do(inp, s)
			}
		case keybind.KeyMatch(X, inp.config.CancelKey, mods, kc):
			if inp.canceled != nil {
				inp.canceled(inp)
			}
			inp.Hide()
		case keybind.KeyMatch(X, inp.config.CompleteKey, mods, kc):
			if len(suggestionSlice) != 0 {
				inp.input.SetString(suggestionSlice[inp.suggestionIndex])
			}
                        inp.suggestionIndex = 0
		case keybind.KeyMatch(X, inp.config.PrevSuggestionKey, mods, kc):
			inp.suggestionIndex--
			if inp.suggestionIndex < 0 {
                          inp.suggestionIndex = len(suggestionSlice) - 1
			}
			text.DrawText(inp.suggestion, inp.theme.Font, inp.theme.FontSize,
				      inp.theme.FontColor, inp.theme.BgColor, suggestionSlice[inp.suggestionIndex])
		case keybind.KeyMatch(X, inp.config.NextSuggestionKey, mods, kc):
			inp.suggestionIndex++
			if inp.suggestionIndex >= len(suggestionSlice) {
                          inp.suggestionIndex = 0
			}
			text.DrawText(inp.suggestion, inp.theme.Font, inp.theme.FontSize,
				      inp.theme.FontColor, inp.theme.BgColor, suggestionSlice[inp.suggestionIndex])
		default:
			inp.input.Add(mods, kc)
			suggestionSlice := inp.suggest(string(inp.input.Text))
			if inp.suggestionIndex >= len(suggestionSlice) {
                          inp.suggestionIndex = 0
			}
			if len(suggestionSlice) != 0 {
				text.DrawText(inp.suggestion, inp.theme.Font, inp.theme.FontSize,
					      inp.theme.FontColor, inp.theme.BgColor, suggestionSlice[inp.suggestionIndex])
			}
		}
	}
	return xevent.KeyPressFun(f)
}

type InputWithSuggestionTheme struct {
	BorderSize  int
	BgColor     render.Color
	InvisibleBgColor  render.Color
	BorderColor render.Color
	Padding     int

	Font      *truetype.Font
	FontSize  float64
	FontColor render.Color
	SuggestionColor render.Color

	InputWidth int
}

var DefaultInputWithSuggestionTheme = &InputWithSuggestionTheme{
	BorderSize:  5,
	BgColor:     render.NewImageColor(color.RGBA{0xff, 0xff, 0xff, 0xff}),
	InvisibleBgColor:     render.NewImageColor(color.RGBA{0xff, 0xff, 0xff, 0x00}),
	BorderColor: render.NewImageColor(color.RGBA{0x0, 0x0, 0x0, 0xff}),
	Padding:     10,

	Font: xgraphics.MustFont(xgraphics.ParseFont(
		bytes.NewBuffer(misc.DataFile("DejaVuSans.ttf")))),
	FontSize:   20.0,
	FontColor:  render.NewImageColor(color.RGBA{0x0, 0x0, 0x0, 0xff}),
	SuggestionColor:  render.NewImageColor(color.RGBA{0xff, 0x0, 0x0, 0xff}),
	InputWidth: 500,
}

type InputWithSuggestionConfig struct {
	CancelKey    string
	BackspaceKey string
	ConfirmKey   string
	CompleteKey  string
	NextSuggestionKey   string
	PrevSuggestionKey   string
}

var DefaultInputWithSuggestionConfig = InputWithSuggestionConfig{
	CancelKey:    "Escape",
	BackspaceKey: "BackSpace",
	ConfirmKey:   "Return",
	CompleteKey  : "Tab",
	NextSuggestionKey   :"Right",
	PrevSuggestionKey   :"Left",
}

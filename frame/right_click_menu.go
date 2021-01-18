package frame

import (
	"bytes"
	"image/color"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/wingo/text"
	"github.com/BurntSushi/wingo/render"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/wingo/misc"
	"github.com/BurntSushi/freetype-go/freetype/truetype"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"log"
)

type RightClickMenu struct {
	Base     Decoration
	Maximize *xwindow.Window
	Minimize *xwindow.Window
	AlwaysOnTop *xwindow.Window
	canceled func(inp *RightClickMenu)
	maximization func(ctx *Context)
	minimization func(ctx *Context)
	always_on_top func(ctx* Context)
}

func NewRightClickMenu(ctx *Context) *RightClickMenu {
	t := &RightClickMenu{}
	cwin := func(p xproto.Window) *xwindow.Window {
		return xwindow.Must(xwindow.Create(ctx.X, p))
	}
	t.Maximize = cwin(ctx.X.RootWin())
	t.Minimize = cwin(ctx.X.RootWin())
	t.AlwaysOnTop = cwin(ctx.X.RootWin())
	var err error

	// Base background
	t.Base, err = CreateDecoration(ctx, TaskbarShape(ctx), ctx.Config.TaskbarBaseColor, 0)
	if err != nil {
		log.Fatal(err)
	}
	t.Base.Window.Map()
	t.Maximize.Map()
	t.Minimize.Map()
	t.AlwaysOnTop.Map()
	mousebind.ButtonPressFun(
		func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
			t.maximization(ctx)
			t.Destroy()
		}).Connect(ctx.X, t.Maximize.Id, ctx.Config.ButtonClick, false, true)
	mousebind.ButtonPressFun(
		func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
			t.minimization(ctx)
			t.Destroy()
		}).Connect(ctx.X, t.Minimize.Id, ctx.Config.ButtonClick, false, true)
	mousebind.ButtonPressFun(
		func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
			t.always_on_top(ctx)
			t.Destroy()
		}).Connect(ctx.X, t.AlwaysOnTop.Id, ctx.Config.ButtonClick, false, true)
	return t
}

func (r *RightClickMenu) Show(x int16, y int16, canceled func(inp *RightClickMenu), maximization func(ctx *Context), minimization func(ctx *Context), always_on_top func(ctx* Context)) {
	r.Maximize.Move(int(x), int(y))
	r.Minimize.Move(int(x), DefaultRightClickTheme.Padding + int(y))
	r.AlwaysOnTop.Move(int(x), 2 * DefaultRightClickTheme.Padding + int(y))
	r.canceled = canceled
	r.maximization = maximization
	r.minimization = minimization
	r.always_on_top = always_on_top
	text.DrawText(r.Maximize, DefaultRightClickTheme.Font, DefaultRightClickTheme.FontSize,
		DefaultRightClickTheme.FontColor, DefaultRightClickTheme.BgColor, "Maximize     ")
	text.DrawText(r.Minimize, DefaultRightClickTheme.Font, DefaultRightClickTheme.FontSize,
		DefaultRightClickTheme.FontColor, DefaultRightClickTheme.BgColor, "Minimize     ")
	text.DrawText(r.AlwaysOnTop, DefaultRightClickTheme.Font, DefaultRightClickTheme.FontSize,
		DefaultRightClickTheme.FontColor, DefaultRightClickTheme.BgColor, "Always On Top")
}

func (r *RightClickMenu) Destroy() {
	r.Maximize.Destroy()
	r.Minimize.Destroy()
	r.AlwaysOnTop.Destroy()
}


type RightClickTheme struct {
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

var DefaultRightClickTheme = &RightClickTheme{
	BorderSize:  5,
	BgColor:     render.NewImageColor(color.RGBA{0xff, 0xff, 0xff, 0xff}),
	InvisibleBgColor:     render.NewImageColor(color.RGBA{0xff, 0xff, 0xff, 0x00}),
	BorderColor: render.NewImageColor(color.RGBA{0x0, 0x0, 0x0, 0xff}),
	Padding:     10,

	Font: xgraphics.MustFont(xgraphics.ParseFont(
		bytes.NewBuffer(misc.DataFile("DejaVuSans.ttf")))),
	FontSize:   10.0,
	FontColor:  render.NewImageColor(color.RGBA{0x0, 0x0, 0x0, 0xff}),
	SuggestionColor:  render.NewImageColor(color.RGBA{0xff, 0x0, 0x0, 0xff}),
	InputWidth: 500,
}

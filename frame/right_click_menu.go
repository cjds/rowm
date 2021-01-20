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
	bBack, bTop, bMid, bBot *xwindow.Window
}

func NewRightClickMenu(ctx *Context) *RightClickMenu {
	t := &RightClickMenu{}
	cwin := func(p xproto.Window) *xwindow.Window {
		return xwindow.Must(xwindow.Create(ctx.X, p))
	}
	rootWin := ctx.X.RootWin()
	t.bBack = cwin(rootWin)
	t.Maximize = cwin(rootWin)
	t.Minimize = cwin(rootWin)
	t.AlwaysOnTop = cwin(rootWin)
	var err error

	// Base background
	t.Base, err = CreateDecoration(ctx, TaskbarShape(ctx), ctx.Config.TaskbarBaseColor, 0)
	if err != nil {
		log.Fatal(err)
	}
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

	t.bTop, t.bMid, t.bBot = cwin(rootWin), cwin(rootWin), cwin(rootWin)
	cclr := func(w *xwindow.Window, clr render.Color) {
		w.Change(xproto.CwBackPixel, clr.Uint32())
	}
	cclr(t.bBack, DefaultRightClickTheme.BgColor)
	cclr(t.bTop, DefaultRightClickTheme.BorderColor)
	cclr(t.bMid, DefaultRightClickTheme.BorderColor)
	cclr(t.bBot, DefaultRightClickTheme.BorderColor)

	t.bBack.Map()
	t.Base.Window.Map()
	t.Maximize.Map()
	t.Minimize.Map()
	t.AlwaysOnTop.Map()
	t.bTop.Map()
	t.bMid.Map()
	t.bBot.Map()

	return t
}

func (r *RightClickMenu) Show(x int16, y int16, canceled func(inp *RightClickMenu), maximization func(ctx *Context), minimization func(ctx *Context), always_on_top func(ctx* Context)) {
	left := int(x)
	top := int(y)
	padding := DefaultRightClickTheme.Padding
	bs := DefaultRightClickTheme.BorderSize
	width := DefaultRightClickTheme.MenuWidth
	r.bBack.MoveResize(left, top, width, top + 5*bs + 2*padding)
	r.bTop.MoveResize(left, top + padding + bs, width, bs)
	r.Maximize.MoveResize(left, top + bs, width, padding)
	r.bMid.MoveResize(left, top + 2*padding + 2*bs, width, bs)
	r.Minimize.Move(left, top + 2*bs + padding)
	r.bBot.MoveResize(left, top + 3*padding + 3*bs, width, bs)
	r.AlwaysOnTop.Move(left, top + 2*bs + 2*padding)

	r.canceled = canceled
	r.maximization = maximization
	r.minimization = minimization
	r.always_on_top = always_on_top
	text.DrawText(r.Maximize, DefaultRightClickTheme.Font, DefaultRightClickTheme.FontSize,
		DefaultRightClickTheme.FontColor, DefaultRightClickTheme.BgColor, "Maximize")
	text.DrawText(r.Minimize, DefaultRightClickTheme.Font, DefaultRightClickTheme.FontSize,
		DefaultRightClickTheme.FontColor, DefaultRightClickTheme.BgColor, "Minimize")
	text.DrawText(r.AlwaysOnTop, DefaultRightClickTheme.Font, DefaultRightClickTheme.FontSize,
		DefaultRightClickTheme.FontColor, DefaultRightClickTheme.BgColor, "On Top  ")
}

func (r *RightClickMenu) Destroy() {
	r.Maximize.Destroy()
	r.Minimize.Destroy()
	r.AlwaysOnTop.Destroy()
	r.bTop.Destroy()
	r.bMid.Destroy()
	r.bBot.Destroy()
}


type RightClickTheme struct {
	BgColor     render.Color
	BorderColor render.Color
	Padding     int
	BorderSize int
	Font      *truetype.Font
	FontSize  float64
	FontColor render.Color
	MenuWidth int
}

var DefaultRightClickTheme = &RightClickTheme{
	BgColor:     render.NewImageColor(color.RGBA{0xff, 0xff, 0xff, 0xff}),
	BorderColor: render.NewImageColor(color.RGBA{0x0, 0x0, 0x0, 0xff}),
	Padding:     17,
	BorderSize: 2,
	Font: xgraphics.MustFont(xgraphics.ParseFont(
		bytes.NewBuffer(misc.DataFile("DejaVuSans.ttf")))),
	FontSize:   15.0,
	FontColor:  render.NewImageColor(color.RGBA{0x0, 0x0, 0x0, 0xff}),
	MenuWidth: 100,
}

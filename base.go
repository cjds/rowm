package main

import (
	"howm/frame"
	"howm/ext"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/BurntSushi/xgb/xproto"
)

func RegisterBaseHooks(ctx *frame.Context) error {
	var err error

	err = keybind.KeyReleaseFun(func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
		xevent.Quit(ctx.X)
	}).Connect(ctx.X, ctx.X.RootWin(), ctx.Config.Shutdown, true)
	if err != nil {
		return err
	}

	err = mousebind.ButtonPressFun(func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
		if ctx.Locked {
			return
		}
	  	ext.Focus(xwindow.New(ctx.X, ctx.X.RootWin()))
		xproto.AllowEvents(ctx.X.Conn(), xproto.AllowReplayPointer, 0)
	}).Connect(ctx.X, ctx.X.RootWin(), ctx.Config.ButtonClick, true, true)
	if err != nil {
		return err
	}

	err = keybind.KeyReleaseFun(func(X *xgbutil.XUtil, ev xevent.KeyReleaseEvent) {
		ctx.Locked = true
		ctx.RaiseLock()
	}).Connect(ctx.X, ctx.X.RootWin(), ctx.Config.Lock, true)
	if err != nil {
		return err
	}

	xevent.MapRequestFun(func(X *xgbutil.XUtil, ev xevent.MapRequestEvent) {
	  frame.NewWindow(ctx, ev)
	  ctx.RaiseLock()
	}).Connect(ctx.X, ctx.X.RootWin())
	return nil
}
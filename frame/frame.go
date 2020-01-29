package frame

import (
	"log"
	"howm/ext"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xcursor"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
)

type DragOrigin struct {
	Container Rect
	Frame Rect
	MouseX int
	MouseY int
}

type Container struct {
	Shape Rect
	Root *Frame
	Expanded *Frame
	DragContext DragOrigin
	Decorations ContainerDecorations
}

type Frame struct {
	Shape Rect
	Window *xwindow.Window
	Container *Container
	Parent, ChildA, ChildB *Frame
	Separator Partition
	Mapped bool
}

type AttachTarget struct {
	Target *Frame
	Type PartitionType
}

func (f *Frame) Traverse(fun func(*Frame)) {
	fun(f)
	if f.ChildA != nil {
		f.ChildA.Traverse(fun)
	}
	if f.ChildB != nil {
		f.ChildB.Traverse(fun)
	}
}

func (f *Frame) Find(fun func(*Frame)bool) *Frame {
	if f == nil || fun(f) {
		return f
	}

	if fA := f.ChildA.Find(fun); fA != nil {
		return fA
	}

	if fB := f.ChildB.Find(fun); fB != nil {
		return fB
	}
	
	return nil
}

func (f *Frame) Root() *Frame {
	z := f
	for {
		if z.Parent != nil {
			z = z.Parent
		} else {
			return z
		}
	}
}

func (f *Frame) Map() {
	f.Traverse(
		func(ft *Frame){
			if ft.Window != nil {
				ft.Window.Map()
			}

			if ft.Separator.Decoration.Window != nil {
				ft.Separator.Decoration.Window.Map()
			}
			ft.Mapped = true
		},
	)
}

func (f *Frame) Close(ctx *Context) {
	wm_protocols, err := xprop.Atm(ctx.X, "WM_PROTOCOLS")
	if err != nil {
		log.Println("xprop wm protocols failed:", err)
		return
	}

	wm_del_win, err := xprop.Atm(ctx.X, "WM_DELETE_WINDOW")
	if err != nil {
		log.Println("xprop delte win failed:", err)
		return
	}

	f.Traverse(func(ft *Frame){
		if ft.IsLeaf() {
			cm, err := xevent.NewClientMessage(32, ft.Window.Id, wm_protocols, int(wm_del_win))
			if err != nil {
				log.Println("new client message failed", err)
				return
			}
			err = xproto.SendEventChecked(ctx.X.Conn(), false, ft.Window.Id, 0, string(cm.Bytes())).Check()
			if err != nil {
				log.Println("Could not send WM_DELETE_WINDOW ClientMessage because:", err)
			}
		}
	})
}

func (f *Frame) IsLeaf() bool {
	return f.ChildA == nil && f.ChildB == nil
}

func (f *Frame) IsRoot() bool {
	return f.Parent == nil
}

func (f *Frame) UnmapSingle() {
	if !f.Mapped {
		return
	}

	if f.Window != nil {
		f.Window.Unmap()
	}
	if f.Separator.Decoration.Window != nil {
		f.Separator.Decoration.Window.Unmap()
	}

	f.Mapped = false
}

func (f *Frame) Unmap() {
	f.Traverse(func(ft *Frame){
		ft.UnmapSingle()
	})
}

func (f *Frame) Destroy(ctx *Context) {
	f.UnmapSingle()
	if f.Window != nil {
		f.Window.Destroy()
		delete(ctx.Tracked, f.Window.Id)
	}
	if f.Separator.Decoration.Window != nil {
		f.Separator.Decoration.Window.Destroy()
	}

	if f.IsRoot() && f.IsLeaf() {
		f.Container.Destroy(ctx)
		return
	}

	if f.IsLeaf() {
		oc := func()*Frame{
			if f.Parent.ChildA == f {
				return f.Parent.ChildB
			} else {
				return f.Parent.ChildA
			}
		}()

		par := oc.Parent
		oc.Parent = par.Parent
		if oc.Parent != nil {
			if oc.Parent.ChildA == par {
				oc.Parent.ChildA = oc
			}
			if oc.Parent.ChildB == par {
				oc.Parent.ChildB = oc
			}
		}
		par.UnmapSingle()
		par.Destroy(ctx)

		oc.MoveResize(ctx)
	}
}

func (f *Frame) Raise(ctx *Context) {
	if f.Window != nil {
		f.Window.Stack(xproto.StackModeAbove)
	}
}

func (f *Frame) RaiseDecoration(ctx *Context) {
	if f.Separator.Decoration.Window != nil {
		f.Separator.Decoration.Window.Stack(xproto.StackModeAbove)
	}
}

func (f *Frame) Focus(ctx *Context) {
	ext.Focus(f.Window)
}

func (f *Frame) FocusRaise(ctx *Context) {
	f.Container.Raise(ctx)
	f.Focus(ctx)
}

func (f *Frame) MoveResize(ctx *Context) {
	f.Traverse(func(ft *Frame){
		ft.Shape = ft.CalcShape(ctx)
		if (ft.Shape.W == 0 || ft.Shape.H == 0) {
			if ft.Mapped {
				ft.Unmap()
			}
		} else {
			if !ft.Mapped{
				ft.Map()
			}
		}

		if ft.IsLeaf() {
			ft.Window.MoveResize(ft.Shape.X, ft.Shape.Y, ft.Shape.W, ft.Shape.H)
		}
		if ft.Separator.Decoration.Window != nil {
			ft.Separator.Decoration.MoveResize(ft.SeparatorShape(ctx))
		}
	})
}

func (f *Frame) CreateSeparatorDecoration(ctx *Context) {
	s := f.SeparatorShape(ctx)
	cursor := ctx.Cursors[xcursor.SBHDoubleArrow]
	if f.Separator.Type == VERTICAL {
		cursor = ctx.Cursors[xcursor.SBVDoubleArrow]
	}

	var err error
	f.Separator.Decoration, err = CreateDecoration(
		ctx, s, ctx.Config.SeparatorColor, uint32(cursor))
	
	if err != nil {
		log.Println(err)
		return
	}

	f.Separator.Decoration.MoveResize(s)
	if err := ext.MapChecked(f.Separator.Decoration.Window); err != nil {
		log.Println("CreateSeparatorDecoration:", f.Separator.Decoration.Window, "could not be mapped", err, s)
	}

	mousebind.Drag(
		ctx.X, f.Separator.Decoration.Window.Id, f.Separator.Decoration.Window.Id, ctx.Config.ButtonDrag, true,
		func(X *xgbutil.XUtil, rX, rY, eX, eY int) (bool, xproto.Cursor) {
			f.Container.DragContext = GenerateDragContext(ctx, f.Container, f, rX, rY)
			f.Container.RaiseFindFocus(ctx)
			return true, ctx.Cursors[xcursor.Circle]
		},
		func(X *xgbutil.XUtil, rX, rY, eX, eY int) {
			if f.Separator.Type == HORIZONTAL {
				f.Separator.Ratio = ext.Clamp((float64(rX) - float64(f.Shape.X)) / float64(f.Shape.W), 0, 1)
			} else {
				f.Separator.Ratio = ext.Clamp((float64(rY) - float64(f.Shape.Y)) / float64(f.Shape.H), 0, 1)
			}
			f.MoveResize(ctx)
		},
		func(X *xgbutil.XUtil, rX, rY, eX, eY int) {
		},
	)
}

func (c *Container) Raise(ctx *Context){
	c.Decorations.ForEach(func(d *Decoration){
		d.Window.Stack(xproto.StackModeAbove)
	})
	c.Root.Traverse(func(f *Frame){
		f.Raise(ctx)
	})
	// Raise decorations separately so we can do overpadding
	c.Root.Traverse(func(f *Frame){
		f.RaiseDecoration(ctx)
	})
}

func (c *Container) ActiveRoot() *Frame {
	if c.Expanded != nil {
		return c.Expanded
	}
	return c.Root
}

func (c *Container) RaiseFindFocus(ctx *Context){
	c.Raise(ctx)
	focus, err := xproto.GetInputFocus(ctx.X.Conn()).Reply()
	if err == nil && ctx.Get(focus.Focus) != nil && ctx.Get(focus.Focus).Container == c {
		return
	}

    focusFrame := c.ActiveRoot().Find(func(ff *Frame)bool{
		return ff.IsLeaf()
	})
	if focusFrame == nil {
		log.Println("RaiseFindFocus: could not find leaf frame")
		return
	}

	focusFrame.Focus(ctx)
}

func (c *Container) Destroy(ctx *Context) {
	c.Decorations.Destroy(ctx)
}

func (c *Container) Map() {
	c.Decorations.Map()
	c.Root.Map()
}

func (c *Container) UpdateFrameMappings() {
	c.Root.Unmap()
	c.ActiveRoot().Map()
}

func (c *Container) MoveResize(ctx *Context, x, y, w, h int) {
	c.Shape = Rect{
		X: x,
		Y: y,
		W: w,
		H: h,
	}
	c.ActiveRoot().MoveResize(ctx)
	c.Decorations.MoveResize(ctx, c.Shape)
}

func (c *Container) MoveResizeShape(ctx *Context, shape Rect) {
	c.MoveResize(ctx, shape.X, shape.Y, shape.W, shape.H)
}

func AttachWindow(ctx *Context, ev xevent.MapRequestEvent) *Frame {
	defer func(){ ctx.AttachPoint = nil }()
	window := ev.Window

	if !ctx.AttachPoint.Target.IsLeaf() {
		log.Println("attach point is not leaf")
		return nil
	}

	ap := ctx.AttachPoint.Target
	ap.Separator.Type = ctx.AttachPoint.Type
	ap.Separator.Ratio = .5
	ap.CreateSeparatorDecoration(ctx)

	// Move current window to child A
	ca := &Frame{
		Window: ap.Window,
		Parent: ap,
		Container: ap.Container,
	}
	ap.ChildA = ca
	ap.Window = nil
	ca.Shape = ca.CalcShape(ctx)
	ctx.Tracked[ca.Window.Id] = ca

	// Add new window as child B
	cb := &Frame{
		Window: xwindow.New(ctx.X, window),
		Parent: ap,
		Container: ap.Container,
	}
	ap.ChildB = cb
	cb.Shape = cb.CalcShape(ctx)
	ctx.Tracked[window] = cb

	if err := ext.MapChecked(cb.Window); err != nil {
		log.Println("NewContainer:", window, "could not be mapped")
		return nil
	}

	err := AddWindowHook(ctx, window)
	if err != nil {
		log.Println("failed to add window hooks", err)
	}

	ap.MoveResize(ctx)

	return cb
}

func NewWindow(ctx *Context, ev xevent.MapRequestEvent) *Frame {
	window := ev.Window
	if existing := ctx.Get(window); existing != nil {
		log.Println("NewWindow:", window, "already tracked")
		return existing
	}

	if ctx.AttachPoint != nil {
		return AttachWindow(ctx, ev)
	}

	defaultShape := Rect{
		X: int(ctx.Config.DefaultShapeRatio.X * float64(ctx.ScreenInfos[0].Width)),
		Y: int(ctx.Config.DefaultShapeRatio.Y * float64(ctx.ScreenInfos[0].Height)),
		W: int(ctx.Config.DefaultShapeRatio.W * float64(ctx.ScreenInfos[0].Width)),
		H: int(ctx.Config.DefaultShapeRatio.H * float64(ctx.ScreenInfos[0].Height)),
	}

	root := &Frame{
		Shape: RootShape(ctx, defaultShape),
		Window: xwindow.New(ctx.X, window),
	}
	root.Window.MoveResize(root.Shape.X, root.Shape.Y, root.Shape.W, root.Shape.H)
	if err := ext.MapChecked(root.Window); err != nil {
		log.Println("NewWindow:", window, "could not be mapped")
		return nil
	}

	c := &Container{
		Shape: defaultShape,
		Root: root,
	}
	root.Container = c

	// Create Decorations
	var err error
	c.Decorations.Close, err = CreateDecoration(
		ctx,
		CloseShape(ctx, c.Shape),
		ctx.Config.CloseColor,
		uint32(ctx.Cursors[xcursor.Dot]),
	)
	ext.Logerr(err)

	c.Decorations.Grab, err = CreateDecoration(
		ctx,
		GrabShape(ctx, c.Shape),
		ctx.Config.GrabColor,
		0,
	)
	ext.Logerr(err)

	c.Decorations.Top, err = CreateDecoration(
		ctx,
		TopShape(ctx, c.Shape),
		ctx.Config.SeparatorColor,
		uint32(ctx.Cursors[xcursor.TopSide]),
	)
	ext.Logerr(err)

	c.Decorations.Bottom, err = CreateDecoration(
		ctx,
		BottomShape(ctx, c.Shape),
		ctx.Config.SeparatorColor,
		uint32(ctx.Cursors[xcursor.BottomSide]),
	)
	ext.Logerr(err)

	c.Decorations.Left, err = CreateDecoration(
		ctx,
		LeftShape(ctx, c.Shape),
		ctx.Config.SeparatorColor,
		uint32(ctx.Cursors[xcursor.LeftSide]),
	)
	ext.Logerr(err)

	c.Decorations.Right, err = CreateDecoration(
		ctx,
		RightShape(ctx, c.Shape),
		ctx.Config.SeparatorColor,
		uint32(ctx.Cursors[xcursor.RightSide]),
	)
	ext.Logerr(err)

	c.Decorations.BottomRight, err = CreateDecoration(
		ctx,
		BottomRightShape(ctx, c.Shape),
		ctx.Config.ResizeColor,
		uint32(ctx.Cursors[xcursor.BottomRightCorner]),
	)
	ext.Logerr(err)

	c.Decorations.BottomLeft, err = CreateDecoration(
		ctx,
		BottomLeftShape(ctx, c.Shape),
		ctx.Config.ResizeColor,
		uint32(ctx.Cursors[xcursor.BottomLeftCorner]),
	)
	ext.Logerr(err)

	c.Decorations.TopRight, err = CreateDecoration(
		ctx,
		TopRightShape(ctx, c.Shape),
		ctx.Config.ResizeColor,
		uint32(ctx.Cursors[xcursor.TopRightCorner]),
	)
	ext.Logerr(err)

	c.Decorations.TopLeft, err = CreateDecoration(
		ctx,
		TopLeftShape(ctx, c.Shape),
		ctx.Config.ResizeColor,
		uint32(ctx.Cursors[xcursor.TopLeftCorner]),
	)
	ext.Logerr(err)

	// Add hooks
	err = c.AddCloseHook(ctx)
	ext.Logerr(err)
	c.AddTopHook(ctx)
	c.AddBottomHook(ctx)
	c.AddLeftHook(ctx)
	c.AddRightHook(ctx)
	c.AddBottomRightHook(ctx)
	c.AddBottomLeftHook(ctx)
	c.AddTopRightHook(ctx)
	c.AddTopLeftHook(ctx)
	c.AddGrabHook(ctx)
	err = AddWindowHook(ctx, window)
	ext.Logerr(err)

	if err != nil {
		log.Println("NewWindow: failed to create container")
		return nil
	}

	// Yay
	c.Map()
	ctx.Tracked[window] = c.Root
	return c.Root
}

func RootShape(context *Context, cShape Rect) Rect {
	return Rect{
		X: cShape.X + context.Config.ElemSize,
		Y: cShape.Y + 2*context.Config.ElemSize,
		W: cShape.W - 2*context.Config.ElemSize,
		H: cShape.H - 3*context.Config.ElemSize,
	}
}

func ContainerShapeFromRoot(ctx *Context, fShape Rect) Rect {
	return Rect{
		X: fShape.X - ctx.Config.ElemSize,
		Y: fShape.Y - 2*ctx.Config.ElemSize,
		W: fShape.W + 2*ctx.Config.ElemSize,
		H: fShape.H + 3*ctx.Config.ElemSize,
	}
}

func (f *Frame) CalcShape(ctx *Context) Rect {
	if f.IsRoot() || f.Container.Expanded == f {
		return RootShape(ctx, f.Container.Shape)
	}

	pShape := func()Rect{
		if f.Parent != nil && f.Container.Expanded == f.Parent {
			return RootShape(ctx, f.Container.Shape)
		} else {
			return f.Parent.Shape
		}
	}()

	isChildA := (f.Parent.ChildA == f)

	WidthA := func()int{
		return ext.IMax(int(float64(pShape.W) * f.Parent.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}
	HeightA := func()int{
		return ext.IMax(int(float64(pShape.H) * f.Parent.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}

	if isChildA {
		if f.Parent.Separator.Type == HORIZONTAL {
			return Rect{
				X: pShape.X,
				Y: pShape.Y,
				W: WidthA(),
				H: pShape.H,
			}
		} else {
			return Rect{
				X: pShape.X,
				Y: pShape.Y,
				W: pShape.W,
				H: HeightA(),
			}
		}
	} else {
		if f.Parent.Separator.Type == HORIZONTAL {
			return Rect{
				X: pShape.X + WidthA() + ctx.Config.ElemSize,
				Y: pShape.Y,
				W: pShape.W - WidthA() - ctx.Config.ElemSize,
				H: pShape.H,
			}
		} else {
			return Rect{
				X: pShape.X,
				Y: pShape.Y + HeightA() + ctx.Config.ElemSize,
				W: pShape.W,
				H: pShape.H - HeightA() - ctx.Config.ElemSize,
			}
		}
	}
}

func (f *Frame) SeparatorShape(ctx *Context) Rect {
	WidthA := func()int{
		return ext.IMax(int(float64(f.Shape.W) * f.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}
	HeightA := func()int{
		return ext.IMax(int(float64(f.Shape.H) * f.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}
	if f.Separator.Type == HORIZONTAL {
		return Rect{
			X: f.Shape.X + WidthA() - ctx.Config.InternalPadding,
			Y: f.Shape.Y - ctx.Config.InternalPadding,
			W: ctx.Config.ElemSize +ctx.Config.InternalPadding,
			H: f.Shape.H  + ctx.Config.InternalPadding,
		}
	} else {
		return Rect{
			X: f.Shape.X - ctx.Config.InternalPadding,
			Y: f.Shape.Y + HeightA() - ctx.Config.InternalPadding,
			W: f.Shape.W + ctx.Config.InternalPadding,
			H: ctx.Config.ElemSize + ctx.Config.InternalPadding,
		}	
	}
}

func AddWindowHook(ctx *Context, window xproto.Window) error {
	xevent.ConfigureRequestFun(
		func(X *xgbutil.XUtil, ev xevent.ConfigureRequestEvent) {
			f := ctx.Get(window)
			if f != nil && f.IsRoot() && f.IsLeaf() {
				fShape := f.Shape
				fShape.X = int(ev.X)
				fShape.Y = int(ev.Y)
				fShape.W = int(ev.Width)
				fShape.H = int(ev.Height)
				cShape := ContainerShapeFromRoot(ctx, fShape)
				cShape.X = ext.IMax(cShape.X, 0)
				cShape.Y = ext.IMax(cShape.Y, 0)
				f.Container.MoveResize(ctx, cShape.X, cShape.Y, cShape.W, cShape.H)
			} else {
				f.MoveResize(ctx)
			}
	}).Connect(ctx.X, window)

	xevent.DestroyNotifyFun(
		func(X *xgbutil.XUtil, ev xevent.DestroyNotifyEvent) {
			f := ctx.Get(window)
			f.Destroy(ctx)
			delete(ctx.Tracked, window)
		}).Connect(ctx.X, window)

	xevent.UnmapNotifyFun(
		func(X *xgbutil.XUtil, ev xevent.UnmapNotifyEvent) {
			f := ctx.Get(window)
			if !f.IsRoot() || !f.IsLeaf() {
				return
			}
			f.Unmap()
		}).Connect(ctx.X, window)
	
	err := mousebind.ButtonPressFun(
		func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
			f := ctx.Get(window)
			f.FocusRaise(ctx)
			xproto.AllowEvents(ctx.X.Conn(), xproto.AllowReplayPointer, 0)
		}).Connect(ctx.X, window, ctx.Config.ButtonClick, true, true)
	ext.Logerr(err)

	err = keybind.KeyPressFun(
		func(X *xgbutil.XUtil, e xevent.KeyPressEvent){
			f := ctx.Get(window)
			if f.IsLeaf() {
				f.Close(ctx)
			}
	  }).Connect(ctx.X, window, ctx.Config.CloseFrame, true)
	ext.Logerr(err)

	err = keybind.KeyPressFun(
		func(X *xgbutil.XUtil, e xevent.KeyPressEvent){
			f := ctx.Get(window)
			if f.Container.Expanded == f {
				f.Container.Expanded = nil
			} else {
				f.Container.Expanded = f
			}
			f.Container.UpdateFrameMappings()
			f.Container.MoveResizeShape(ctx, f.Container.Shape)
			f.Container.RaiseFindFocus(ctx)
	  }).Connect(ctx.X, window, ctx.Config.ToggleExpandFrame, true)
	ext.Logerr(err)

	return err
}

func GenerateDragContext(ctx *Context, c *Container, f *Frame, mouseX, mouseY int) DragOrigin {
	dc := DragOrigin{}
	if c != nil {
		dc.Container = c.Shape
	}
	if f != nil {
		dc.Frame = f.Shape
	}
	dc.MouseX = mouseX
	dc.MouseY = mouseY
	return dc
}

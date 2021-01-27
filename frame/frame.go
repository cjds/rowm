package frame

import (
	"container/list"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xcursor"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/levavakian/rowm/ext"
	"log"
)

// Frame represents a node in the tree like structure of panels in each container.
// A frame can either be a leaf node, which has a user created window to display,
// or it can display a seperator decoration that allows the resizing of its child frames.
type Frame struct {
	Shape                  Rect
	Window                 *xwindow.Window
	Container              *Container
	Parent, ChildA, ChildB *Frame
	Separator              Partition
	Mapped                 bool
}

// Traverse will visit every frame in the tree starting at the input frame.
func (f *Frame) Traverse(fun func(*Frame)) {
	fun(f)
	if f.ChildA != nil {
		f.ChildA.Traverse(fun)
	}
	if f.ChildB != nil {
		f.ChildB.Traverse(fun)
	}
}

// Find will find the first frame that fulfills the predicate and is a descendent of the input frame.
func (f *Frame) Find(fun func(*Frame) bool) *Frame {
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

// FindNearest will run BFS on the tree the input frame is in to find a frame that fulfills the predicate.
func (f *Frame) FindNearest(fun func(*Frame) bool) *Frame {
	visited := make(map[*Frame]bool)
	nbrs := list.New()

	pb := func(fpb *Frame) {
		if v, _ := visited[fpb]; !v {
			nbrs.PushBack(fpb)
		}
	}

	pb(f)

	for nbrs.Len() > 0 {
		pop := nbrs.Front()
		fr := pop.Value.(*Frame)
		nbrs.Remove(pop)
		visited[fr] = true

		if fun(fr) {
			return fr
		}

		if fr.ChildA != nil {
			pb(fr.ChildA)
		}

		if fr.ChildB != nil {
			pb(fr.ChildB)
		}

		if fr.Parent != nil {
			pb(fr.Parent)
		}
	}
	return nil
}

// Returns the first node in a subtree, root must be a parent of f or nil
func (f *Frame) GetLeftmostFrameInSubtree() *Frame {
	curr := f
	for curr.ChildA != nil {
		curr = curr.ChildA
	}
	return curr
}

// Returns the last node in a subtree, root must be a parent of f or nil
func (f *Frame) GetRightmostFrameInSubtree() *Frame {
	curr := f
	for curr.ChildB != nil {
		curr = curr.ChildB
	}
	return curr
}

// FindNext will find the next (top down, left to right) leaf element in the tree that fulfills a predicate
// You can provide a root frame which will constrain the search to a subtree, must be a parent of f or nil
// If reversed, it will go backwards through the tree
func (f *Frame) FindNextLeaf(fun func(*Frame) bool, reversed bool, root *Frame) *Frame {
	if f.IsOrphan() {
		return nil
	}

	start := f

	// Early exit if frame is not a child of root at any point
	found := func() bool {
		parfind := start
		for parfind != nil {
			if parfind == root {
				return true
			}

			parfind = parfind.Parent
		}
		return false
	}()
	if !found {
		return nil
	}
	// If we're not starting on a leaf, force it to be the first leaf in the tree
	if !start.IsLeaf() {
		start = start.GetLeftmostFrameInSubtree()
	}
	// Early exit if start satisfies predicate already
	if start != f && fun(start) {
		return start
	}

	leftmost := root.GetLeftmostFrameInSubtree()
	rightmost := root.GetRightmostFrameInSubtree()
	nextLeaf := func(fc *Frame) *Frame {
		if !reversed && fc == rightmost {
			return leftmost
		} else if reversed && fc == leftmost {
			return rightmost
		}

		for {
			if fc == nil || (fc == root && !fc.IsLeaf()) {
				return nil
			}
			if !reversed && fc == fc.Parent.ChildA {
				return fc.Parent.ChildB.GetLeftmostFrameInSubtree()
			} else if reversed && fc == fc.Parent.ChildB {
				return fc.Parent.ChildA.GetRightmostFrameInSubtree()
			}
			fc = fc.Parent
		}
	}

	curr := nextLeaf(start)
	for {
		if curr == nil {
			break
		}

		if curr == start {
			break
		}

		if fun(curr) {
			return curr
		}

		curr = nextLeaf(curr)
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
		func(ft *Frame) {
			if ft.Mapped {
				return
			}

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

func (f *Frame) UnmapSingle(ctx *Context) {
	if !f.Mapped {
		return
	}

	if f.Window != nil {
		f.Window.Unmap()
		// Keep track of how many unmaps we've sent since we get a notification
		// every time we unmap something internally, but we only care about external ones.
		ctx.UnmapCounter[f.Window.Id]++
	}
	if f.Separator.Decoration.Window != nil {
		f.Separator.Decoration.Window.Unmap()
	}

	f.Mapped = false
}

func (f *Frame) Unmap(ctx *Context) {
	f.Traverse(func(ft *Frame) {
		ft.UnmapSingle(ctx)
	})
}

func (f *Frame) Close(ctx *Context) {
	_, present := ctx.AlwaysOnTop[f.Window.Id]
	if present{
		delete(ctx.AlwaysOnTop, f.Window.Id )
	}
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

	f.Traverse(func(ft *Frame) {
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

func (f *Frame) IsOrphan() bool {
	return f.Container == nil
}

// Orphan removes a frame from its container and reorganizes the tree to fill the gap.
func (f *Frame) Orphan(ctx *Context) {
	if f.Container == nil {
		log.Println("orphan called on already orphaned frame")
		return
	}
	if !f.IsLeaf() {
		log.Println("can't orphan non leaf frame")
		return
	}
	f.UnmapSingle(ctx)
	defer func() {
		f.Parent = nil
		f.Container = nil
	}()

	if f.IsRoot() {
		f.Container.Destroy(ctx)
		return
	}

	if f.Container.Expanded == f {
		f.Container.Expanded = nil
		f.Container.UpdateFrameMappings(ctx)
	}

	oc := func() *Frame {
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
	if par.IsRoot() {
		oc.Container.Root = oc
	}
	par.Isolate(ctx)
	par.Destroy(ctx)
	if oc.Mapped {
		oc.MoveResize(ctx)
	}
	ctx.Taskbar.UpdateContainer(ctx, f.Container)
}

func (f *Frame) Isolate(ctx *Context) {
	f.Parent = nil
	f.ChildA = nil
	f.ChildB = nil
	f.Container = nil
}

func (f *Frame) Destroy(ctx *Context) {
	f.UnmapSingle(ctx)
	f.Orphan(ctx)
	if f.Window != nil {
		f.Window.Destroy()
		delete(ctx.Tracked, f.Window.Id)
	}
	if f.Separator.Decoration.Window != nil {
		f.Separator.Decoration.Window.Destroy()
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
	leaf := f.Find(func(ff *Frame) bool {
		return ff.IsLeaf()
	})
	if leaf != nil {
		ext.Focus(leaf.Window)
		ctx.LastKnownFocused = leaf.Window.Id
		_, _, ctx.LastKnownFocusedScreen = ctx.GetScreenForShape(leaf.Container.Shape)
	}
}

func (f *Frame) FocusRaise(ctx *Context) {
	if f.IsOrphan() {
		log.Println("tried to raise an orphan")
		return
	}
	f.Container.Raise(ctx)
	f.Focus(ctx)
}

// MoveResize will cascade down changes in shape down the tree.
func (f *Frame) MoveResize(ctx *Context) {
	if f.IsOrphan() && f.IsRoot() {
		log.Println("tried to move resize orphaned root")
		return
	}
	f.Traverse(func(ft *Frame) {
		ft.Shape = ft.CalcShape(ctx)
		if ft.Shape.W == 0 || ft.Shape.H == 0 {
			if ft.Mapped {
				ft.Unmap(ctx)
			}
		} else {
			if !ft.Mapped {
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
				f.Separator.Ratio = ext.Clamp((float64(rX)-float64(f.Shape.X))/float64(f.Shape.W), 0, 1)
			} else {
				f.Separator.Ratio = ext.Clamp((float64(rY)-float64(f.Shape.Y))/float64(f.Shape.H), 0, 1)
			}
			f.MoveResize(ctx)
		},
		func(X *xgbutil.XUtil, rX, rY, eX, eY int) {
			f.Container.RaiseFindFocus(ctx)
		},
	)
}

// CalcShape returns the shape a frame should be based off of its container and parent
func (f *Frame) CalcShape(ctx *Context) Rect {
	if f == f.Container.ActiveRoot() {
		return RootShape(ctx, f.Container)
	}

	pShape := func() Rect {
		if f.Parent != nil && f.Container.Expanded == f.Parent {
			return RootShape(ctx, f.Container)
		} else {
			return f.Parent.Shape
		}
	}()

	isChildA := (f.Parent.ChildA == f)

	WidthA := func() int {
		return ext.IMax(int(float64(pShape.W)*f.Parent.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}
	HeightA := func() int {
		return ext.IMax(int(float64(pShape.H)*f.Parent.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
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
	WidthA := func() int {
		return ext.IMax(int(float64(f.Shape.W)*f.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}
	HeightA := func() int {
		return ext.IMax(int(float64(f.Shape.H)*f.Separator.Ratio), ctx.Config.ElemSize) - ctx.Config.ElemSize
	}
	if f.Separator.Type == HORIZONTAL {
		return Rect{
			X: f.Shape.X + WidthA() - ctx.Config.InternalPadding,
			Y: f.Shape.Y - ctx.Config.InternalPadding,
			W: ctx.Config.ElemSize + ctx.Config.InternalPadding,
			H: f.Shape.H + ctx.Config.InternalPadding,
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

// AddWindowHook registers callbacks for window related events.
func AddWindowHook(ctx *Context, window xproto.Window) error {
	xevent.ConfigureRequestFun(
		func(X *xgbutil.XUtil, ev xevent.ConfigureRequestEvent) {
			f := ctx.Get(window)
			if f != nil && !f.IsOrphan() && f.IsRoot() && f.IsLeaf() {
				fShape := f.Shape
				fShape.X = int(ev.X)
				fShape.Y = int(ev.Y)
				fShape.W = int(ev.Width)
				fShape.H = int(ev.Height)
				cShape := ContainerShapeFromRoot(ctx, fShape)
				cShape.X = ext.IMax(cShape.X, 0)
				cShape.Y = ext.IMax(cShape.Y, 0)
				f.Container.MoveResize(ctx, cShape.X, cShape.Y, cShape.W, cShape.H)
			} else if !f.IsOrphan() {
				f.MoveResize(ctx)
			}
			ctx.RaiseLock()
		}).Connect(ctx.X, window)

	xevent.ClientMessageFun(
		func(X *xgbutil.XUtil, ev xevent.ClientMessageEvent) {
			name, err := xprop.AtomName(X, ev.Type)
			if err != nil {
				log.Println(err)
				return
			}
			switch name {
			case "_NET_WM_STATE":
				f := ctx.Get(window)
				if f.IsOrphan() {
					return
				}
				// TODO: This is a dirty hack, instead of properly implementing ewmh
				// we toggle minimazation state to get internal media players to resize
				f.Container.ChangeMinimizationState(ctx)
				f.Container.ChangeMinimizationState(ctx)
				ctx.RaiseLock() // in case the fullscreen message happens in background
			}
		}).Connect(ctx.X, window)

	xevent.UnmapNotifyFun(
		func(X *xgbutil.XUtil, ev xevent.UnmapNotifyEvent) {
			// Keep track of how many unmaps we've received since we get a notification
			// every time we unmap something internally, but we only care about external ones.
			if ctx.UnmapCounter[window] > 0 {
				ctx.UnmapCounter[window]--
				return
			}

			f := ctx.Get(window)
			if ctx.GetFocusedFrame() == f {
				nf := f.FindNearest(func(fr *Frame) bool {
					return fr != f && fr.Mapped && fr.IsLeaf()
				})
				if nf != nil {
					nf.Focus(ctx)
				} else {
					ext.Focus(xwindow.New(ctx.X, ctx.X.RootWin()))
				}
			}

			f.Orphan(ctx)
			ctx.RaiseLock()
		}).Connect(ctx.X, window)

	xevent.DestroyNotifyFun(
		func(X *xgbutil.XUtil, ev xevent.DestroyNotifyEvent) {
			f := ctx.Get(window)
			f.Destroy(ctx)
			delete(ctx.Tracked, window)
			ctx.RaiseLock()
		}).Connect(ctx.X, window)

	err := mousebind.ButtonPressFun(
		func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			f.FocusRaise(ctx)
			xproto.AllowEvents(ctx.X.Conn(), xproto.AllowReplayPointer, 0)
		}).Connect(ctx.X, window, ctx.Config.ButtonClick, true, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsLeaf() {
				f.Close(ctx)
			}
		}).Connect(ctx.X, window, ctx.Config.CloseFrame.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			ctx.Yanked = &Yank{Window: f.Window.Id, Container: nil}
		}).Connect(ctx.X, window, ctx.Config.CutSelectFrame, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			ctx.Yanked = &Yank{Window: 0, Container: f.Container}
		}).Connect(ctx.X, window, ctx.Config.CutSelectContainer, true)
	ext.Logerr(err)

	yankAttach := func(window xproto.Window, partition PartitionType) {
		if ctx.Locked {
			return
		}

		if ctx.Yanked == nil {
			return
		}
		defer func() { ctx.Yanked = nil }()

		target := ctx.Get(window)
		if target == nil {
			log.Println("could not find target window for yank attach")
		}
		source := func() *Frame {
			if ctx.Yanked.Container != nil && ctx.Yanked.Container.Root != nil {
				if ctx.Yanked.Container == target.Container {
					log.Println("tried to yank container into itself")
					return nil
				}
				s := ctx.Yanked.Container.Root
				s.Unmap(ctx)
				ctx.Yanked.Container.Destroy(ctx)
				return s
			} else {
				s := ctx.Get(ctx.Yanked.Window)
				if s == target {
					log.Println("tried to yank frame into itself")
					return nil
				}
				if s != nil {
					s.Orphan(ctx)
				}
				return s
			}
		}()
		if source == nil {
			log.Println("could not find source window for yank attach")
			return
		}

		AttachWindow(ctx, target, partition, 0, source)
	}

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			yankAttach(window, HORIZONTAL)
		}).Connect(ctx.X, window, ctx.Config.CopySelectHorizontal.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			yankAttach(window, VERTICAL)
		}).Connect(ctx.X, window, ctx.Config.CopySelectVertical.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}

			f := ctx.Get(window)
			if f.IsLeaf() && !f.IsRoot() {
				f.Orphan(ctx)
				NewWindow(ctx, f.Window.Id)
			}
		}).Connect(ctx.X, window, ctx.Config.PopFrame.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
		if ctx.Locked {
			return
		}

		f := ctx.Get(window)
		if f.IsOrphan() {
			return
		}
		if !f.Container.Hidden {
			f.Container.ChangeMinimizationState(ctx)
		}
	}).Connect(ctx.X, window, ctx.Config.Minimize, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			if f.Container.Expanded == f || f.IsRoot() {
				f.Container.Expanded = nil
			} else {
				f.Container.Expanded = f
			}
			f.Container.UpdateFrameMappings(ctx)
			f.Focus(ctx)
			f.Container.MoveResizeShape(ctx, f.Container.Shape)
		}).Connect(ctx.X, window, ctx.Config.ToggleExpandFrame.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			f.Container.Decorations.Hidden = !f.Container.Decorations.Hidden
			f.Container.UpdateFrameMappings(ctx)
			f.Container.MoveResizeShape(ctx, f.Container.Shape)
		}).Connect(ctx.X, window, ctx.Config.ToggleExternalDecorator, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			screen, _, _ := ctx.GetScreenForShape(f.Container.Shape)
			f.Container.MoveResizeShape(ctx, ctx.DefaultShapeForScreen(screen))
		}).Connect(ctx.X, window, ctx.Config.ResetSize, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			screen, _, _ := ctx.GetScreenForShape(f.Container.Shape)
			if f.Container.Shape == AnchorShape(ctx, screen, FULL) {
				s := AnchorShape(ctx, screen, TOP)
				f.Container.MoveResizeShape(ctx, s)
			} else if f.Container.Shape == AnchorShape(ctx, screen, TOP) {
				raised := screen
				raised.Y = raised.Y - raised.H
				if nscreen, overlap, _ := ctx.GetScreenForShape(raised); overlap > 0 && nscreen != screen {
					f.Container.MoveResizeShape(ctx, AnchorShape(ctx, nscreen, BOTTOM))
				}
			} else if f.Container.Shape == AnchorShape(ctx, screen, BOTTOM) {
				f.Container.MoveResizeShape(ctx, f.Container.RestingShape(ctx, screen))
			} else {
				f.Container.MoveResizeShape(ctx, AnchorShape(ctx, screen, FULL))
			}
		}).Connect(ctx.X, window, ctx.Config.WindowUp.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			screen, _, _ := ctx.GetScreenForShape(f.Container.Shape)
			if f.Container.Shape == AnchorShape(ctx, screen, FULL) || f.Container.Shape == AnchorShape(ctx, screen, TOP) {
				f.Container.MoveResizeShape(ctx, f.Container.RestingShape(ctx, screen))
			} else if f.Container.Shape == AnchorShape(ctx, screen, BOTTOM) {
				lowered := screen
				lowered.Y = lowered.Y + lowered.H
				if nscreen, overlap, _ := ctx.GetScreenForShape(lowered); overlap > 0 && nscreen != screen {
					f.Container.MoveResizeShape(ctx, AnchorShape(ctx, nscreen, TOP))
				}
			} else {
				f.Container.MoveResizeShape(ctx, AnchorShape(ctx, screen, BOTTOM))
			}
		}).Connect(ctx.X, window, ctx.Config.WindowDown.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			screen, _, _ := ctx.GetScreenForShape(f.Container.Shape)
			if f.Container.Shape == AnchorShape(ctx, screen, RIGHT) {
				f.Container.MoveResizeShape(ctx, f.Container.RestingShape(ctx, screen))
			} else if f.Container.Shape == AnchorShape(ctx, screen, LEFT) {
				lefted := screen
				lefted.X = lefted.X - lefted.W
				if nscreen, overlap, _ := ctx.GetScreenForShape(lefted); overlap > 0 && nscreen != screen {
					f.Container.MoveResizeShape(ctx, AnchorShape(ctx, nscreen, RIGHT))
				}
			} else {
				f.Container.MoveResizeShape(ctx, AnchorShape(ctx, screen, LEFT))
			}
		}).Connect(ctx.X, window, ctx.Config.WindowLeft.Data, true)
	ext.Logerr(err)

	err = keybind.KeyReleaseFun(
		func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}
			f := ctx.Get(window)
			if f.IsOrphan() {
				return
			}
			screen, _, _ := ctx.GetScreenForShape(f.Container.Shape)
			if f.Container.Shape == AnchorShape(ctx, screen, LEFT) {
				f.Container.MoveResizeShape(ctx, f.Container.RestingShape(ctx, screen))
			} else if f.Container.Shape == AnchorShape(ctx, screen, RIGHT) {
				righted := screen
				righted.X = righted.X + righted.W
				if nscreen, overlap, _ := ctx.GetScreenForShape(righted); overlap > 0 && nscreen != screen {
					f.Container.MoveResizeShape(ctx, AnchorShape(ctx, nscreen, LEFT))
				}
			} else {
				f.Container.MoveResizeShape(ctx, AnchorShape(ctx, screen, RIGHT))
			}
		}).Connect(ctx.X, window, ctx.Config.WindowRight.Data, true)
	ext.Logerr(err)

	for k, v := range ctx.Config.GotoKeys {
		kref := k // capture separately so we can use in closure
		vref := v // capture separately so we can use in closure
		err = keybind.KeyReleaseFun(func(X *xgbutil.XUtil, e xevent.KeyReleaseEvent) {
			if ctx.Locked {
				return
			}

			ctx.Gotos[vref] = window
		}).Connect(ctx.X, window, kref, true)
		ext.Logerr(err)
	}

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

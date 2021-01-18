package frame

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/levavakian/rowm/ext"
	"log"
)

func AttachWindow(ctx *Context, target *Frame, partitition PartitionType, window xproto.Window, existing *Frame) *Frame {
	if !target.IsLeaf() {
		log.Println("attach point is not leaf")
		return nil
	}

	ap := target
	ap.Separator.Type = partitition
	ap.Separator.Ratio = .5
	ap.CreateSeparatorDecoration(ctx)

	// Move current window to child A
	ca := &Frame{
		Window:    ap.Window,
		Parent:    ap,
		Container: ap.Container,
	}
	ap.ChildA = ca
	ap.Window = nil
	ca.Shape = ca.CalcShape(ctx)
	ctx.Tracked[ca.Window.Id] = ca

	// Add new window as child B
	cb := func() *Frame {
		if existing == nil {
			nf := &Frame{
				Window:    xwindow.New(ctx.X, window),
				Parent:    ap,
				Container: ap.Container,
			}
			ap.ChildB = nf
			nf.Shape = nf.CalcShape(ctx)
			nf.Window.Stack(xproto.StackModeAbove)
			ctx.Tracked[window] = nf

			if err := ext.MapChecked(nf.Window); err != nil {
				log.Println("NewContainer:", window, "could not be mapped")
				return nil
			}

			err := AddWindowHook(ctx, window)
			if err != nil {
				log.Println("failed to add window hooks", err)
			}
			return nf
		} else {
			nf := existing
			nf.Parent = ap
			ap.ChildB = nf
			nf.Traverse(func(ft *Frame) {
				ft.Container = ap.Container
			})
			return nf
		}
	}()
	cb.Map()

	ap.MoveResize(ctx)
	ap.Container.Raise(ctx)
	cb.Find(func(ff *Frame) bool { return ff.IsLeaf() }).Focus(ctx)
	return cb
}

func NewWindow(ctx *Context, window xproto.Window) *Frame {
	existing := ctx.Get(window)
	if existing != nil && existing.Container != nil {
		return existing
	}

	if ctx.AttachPoint != nil {
		defer func() { ctx.AttachPoint = nil }()
		return AttachWindow(ctx, ctx.AttachPoint.Target, ctx.AttachPoint.Type, window, nil)
	}

	// Create container and root frame
	c := &Container{
		Shape: ctx.DefaultShapeForScreen(ctx.LastFocusedScreen()),
	}

	root := func() *Frame {
		if existing != nil {
			existing.Container = c
			existing.Shape = RootShape(ctx, c)
			return existing
		}
		return &Frame{
			Shape:     RootShape(ctx, c),
			Window:    xwindow.New(ctx.X, window),
			Container: c,
		}
	}()
	root.Window.MoveResize(root.Shape.X, root.Shape.Y, root.Shape.W, root.Shape.H)
	if err := ext.MapChecked(root.Window); err != nil {
		log.Println("NewWindow:", window, "could not be mapped")
	}
	if existing == nil {
		err := AddWindowHook(ctx, window)
		if err != nil {
			log.Println(err)
		}
	}

	c.Root = root

	// Create window decorations and hook up callbacks
	err := GeneratePieces(ctx, c)
	ext.Logerr(err)

	if err != nil {
		log.Println("NewWindow: failed to create container")
		return nil
	}

	c.Map()
	ctx.Tracked[window] = c.Root
	ctx.Containers[c] = struct{}{}
	ctx.Taskbar.UpdateContainer(ctx, c)
	c.Raise(ctx)
	c.Root.Focus(ctx)
	return c.Root
}

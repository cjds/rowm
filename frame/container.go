package frame

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/levavakian/rowm/ext"
	"log"
	"time"
)

type DragOrigin struct {
	Container Rect
	Frame     Rect
	MouseX    int
	MouseY    int
}

// Container represents the wrapping decorations around a frame tree.
// It keeps track of size, position, and minimzation state for the tree.
type Container struct {
	Shape               Rect
	Root                *Frame
	Expanded            *Frame
	DragContext         DragOrigin
	Decorations         ContainerDecorations
	Hidden              bool
	LastUnanchoredShape Rect
	LastGrabTime        time.Time
}

func (c *Container) Raise(ctx *Context) {
	c.Decorations.ForEach(func(d *Decoration) {
		d.Window.Stack(xproto.StackModeAbove)
	})
	c.Root.Traverse(func(f *Frame) {
		f.Raise(ctx)
	})
	// Raise decorations separately so we can do overpadding
	c.Root.Traverse(func(f *Frame) {
		f.RaiseDecoration(ctx)
	})
	ctx.Taskbar.Raise(ctx)

	// Raise the always on top windows
	if !ctx.Locked {
		for _, a := range ctx.AlwaysOnTop {
			if c != a {
			a.Raise(ctx)
			}
		}
	}
}

func (c *Container) ActiveRoot() *Frame {
	if c.Expanded != nil {
		return c.Expanded
	}
	return c.Root
}

func (c *Container) RaiseFindFocus(ctx *Context) {
	c.Raise(ctx)
	if ff := ctx.GetFocusedFrame(); ff != nil && ff.Container == c {
		ff.Focus(ctx)
		return
	}

	focusFrame := c.ActiveRoot().Find(func(ff *Frame) bool {
		return ff.IsLeaf()
	})
	if focusFrame == nil {
		log.Println("RaiseFindFocus: could not find leaf frame")
		return
	}

	focusFrame.Focus(ctx)
}

// Returns the last shape the container had when it was unanchored
func (c *Container) RestingShape(ctx *Context, screen Rect) Rect {
	restingScreen, _, _ := ctx.GetScreenForShape(c.LastUnanchoredShape)
	if c.LastUnanchoredShape != (Rect{}) && restingScreen == screen {
		return c.LastUnanchoredShape
	} else {
		return ctx.DefaultShapeForScreen(screen)
	}
}

func (c *Container) Destroy(ctx *Context) {
	c.Decorations.Destroy(ctx)
	c.Root.Traverse(func(ft *Frame) {
		ft.Container = nil
	})
	c.Root = nil
	delete(ctx.Containers, c)
	ctx.Taskbar.RemoveContainer(ctx, c)
	xwindow.New(ctx.X, ctx.X.RootWin()).Focus()
}

func (c *Container) Map() {
	c.Decorations.Map()
	c.Root.Map()
}

func (c *Container) ChangeMinimizationState(ctx *Context) {
	c.Hidden = !c.Hidden
	c.UpdateFrameMappings(ctx)
	if !c.Hidden {
		c.RaiseFindFocus(ctx)
	} else {
		ext.Focus(xwindow.New(ctx.X, ctx.X.RootWin()))
	}
	ctx.Taskbar.UpdateContainer(ctx, c)
}

func (c *Container) ChangeMaximizationState(ctx *Context) {
	screen, _, _ := ctx.GetScreenForShape(c.Shape)
	s := AnchorShape(ctx, screen, FULL)
	if c.Shape == s {
		c.MoveResizeShape(ctx, ctx.DefaultShapeForScreen(screen))
	} else {
		c.MoveResizeShape(ctx, s)
	}
}

func (c *Container) ChangeAlwaysOnTopState(ctx *Context, turn_on bool) {
	if turn_on {
		ctx.AlwaysOnTop[c.Root.Window.Id] = c
	} else {
		delete(ctx.AlwaysOnTop, c.Root.Window.Id)
	}
}

func (c *Container) UpdateFrameMappings(ctx *Context) {
	if c.Hidden {
		c.Root.Unmap(ctx)
		c.Decorations.Unmap()
		return
	}

	if !c.Decorations.Hidden {
		c.Decorations.Map()
	} else {
		c.Decorations.Unmap()
	}

	if c.Root != c.ActiveRoot() {
		c.Root.Unmap(ctx)
	}
	c.ActiveRoot().Map()
}

func (c *Container) MoveResize(ctx *Context, x, y, w, h int) {
	shape := Rect{
		X: x,
		Y: y,
		W: w,
		H: h,
	}
	screen, _, _ := ctx.GetScreenForShape(c.Shape)
	if opt := AnchorMatch(ctx, screen, c.Shape); opt == NONE {
		c.LastUnanchoredShape = c.Shape
	}

	c.MoveResizeShape(ctx, shape)
}

func (c *Container) MoveResizeShape(ctx *Context, shape Rect) {
	c.Shape = shape
	c.ActiveRoot().MoveResize(ctx)
	c.Decorations.MoveResize(ctx, c.Shape)
}

func RootShape(ctx *Context, c *Container) Rect {
	if c.Decorations.Hidden {
		return c.Shape
	}
	return Rect{
		X: c.Shape.X + ctx.Config.ElemSize,
		Y: c.Shape.Y + 2*ctx.Config.ElemSize,
		W: c.Shape.W - 2*ctx.Config.ElemSize,
		H: c.Shape.H - 3*ctx.Config.ElemSize,
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

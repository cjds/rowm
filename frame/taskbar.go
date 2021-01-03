package frame

import (
	"fmt"
	"github.com/BurntSushi/wingo/prompt"
	"github.com/BurntSushi/wingo/render"
	"github.com/BurntSushi/wingo/text"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
	"github.com/distatus/battery"
	"github.com/levavakian/rowm/ext"
	"log"
	"time"
)

type History struct {
	LastBattery      int
	LastBatteryState string
}

type Taskbar struct {
	Base     Decoration
	TimeWin  *xwindow.Window
	BatWin   *xwindow.Window
	Hidden   bool
	Scroller *ElementScroller
	History  History
}

type Element struct {
	Container  *Container
	Window     *xwindow.Window
	MinWin     *xwindow.Window
	Prev, Next *Element
	Active     bool
}

type ElementScroller struct {
	Elements                              map[*Container]*Element
	Back                                  *Element
	Front                                 *Element
	StartingIdx                           int
	CanFit                                int
	ShiftLeftInactive, ShiftRightInactive *xwindow.Window
	ShiftLeftActive, ShiftRightActive     *xwindow.Window
}

func NewTaskbar(ctx *Context) *Taskbar {
	t := &Taskbar{
		History: History{
			LastBattery:      100,
			LastBatteryState: "∘",
		},
	}
	var err error

	// Base background
	t.Base, err = CreateDecoration(ctx, TaskbarShape(ctx), ctx.Config.TaskbarBaseColor, 0)
	if err != nil {
		log.Fatal(err)
	}
	t.Base.Window.Map()

	// Time
	s := TimeShape(ctx)
	win, err := xwindow.Generate(ctx.X)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	win.Create(ctx.X.RootWin(), s.X, s.Y, s.W, s.H, 0)
	win.Map()
	t.TimeWin = win

	// Battery
	s = BatShape(ctx)
	win, err = xwindow.Generate(ctx.X)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	win.Create(ctx.X.RootWin(), s.X, s.Y, s.W, s.H, 0)
	win.Map()
	t.BatWin = win

	// Scroller
	t.Scroller = NewElementScroller(ctx)

	// Initial render
	t.Update(ctx)
	return t
}

func ShouldActivate(idx int, start int, canfit int) bool {
	if idx < start {
		return false
	}
	if idx >= (start + canfit) {
		return false
	}
	return true
}

func (t *Taskbar) UpdateContainer(ctx *Context, c *Container) {
	var err error
	if c.Root == nil {
		log.Println("wanted to update childless container")
		return
	}

	elem, ok := t.Scroller.Elements[c]
	if !ok {
		active := ShouldActivate(len(t.Scroller.Elements), t.Scroller.StartingIdx, t.Scroller.CanFit)
		offset := 0
		if active {
			offset = len(t.Scroller.Elements) - t.Scroller.StartingIdx
		}
		elem = NewElement(ctx, c, offset)
		elem.Active = active
		t.Scroller.Elements[c] = elem
		if t.Scroller.Front == nil {
			t.Scroller.Front = elem
			t.Scroller.Back = elem
		} else {
			elem.Prev = t.Scroller.Back
			t.Scroller.Back = elem
			elem.Prev.Next = elem
		}
	}

	f := c.Root.Find(func(fr *Frame) bool {
		return fr.IsLeaf()
	})
	if f == nil {
		log.Println("could not find representative leaf")
		return
	}

	ximg, err := xgraphics.FindIcon(ctx.X, f.Window.Id, ctx.Config.TaskbarElementShape.W, ctx.Config.TaskbarElementShape.H)
	if err != nil {
		log.Println(err)
		ximg = ctx.DummyIcon
	}

	ximg.XSurfaceSet(elem.Window.Id)
	ximg.XDraw()
	ximg.XPaint(elem.Window.Id)

	elem.UpdateMapping(ctx)
	t.Scroller.UpdateMappings(ctx)
}

func (t *Taskbar) RemoveContainer(ctx *Context, c *Container) {
	elem, ok := t.Scroller.Elements[c]
	if !ok {
		return
	}

	elem.Unmap()
	elem.Destroy()

	if elem.Prev != nil {
		elem.Prev.Next = elem.Next
	}
	if elem.Next != nil {
		elem.Next.Prev = elem.Prev
	}
	if elem == t.Scroller.Front {
		t.Scroller.Front = elem.Next
	}
	if elem == t.Scroller.Back {
		t.Scroller.Back = elem.Prev
	}
	delete(t.Scroller.Elements, c)

	t.Scroller.ShiftAndActivate(ctx)
}

func NewElementScroller(ctx *Context) *ElementScroller {
	canFit := CalcCanFit(ctx)
	es := &ElementScroller{
		Elements: make(map[*Container]*Element),
		CanFit:   canFit,
	}
	var err error
	dec, err := CreateDecoration(ctx, LeftSelectorShape(ctx), ctx.Config.TaskbarSlideInactiveColor, 0)
	es.ShiftLeftInactive = dec.Window
	if err != nil {
		log.Fatal(err)
		return nil
	}
	dec, err = CreateDecoration(ctx, LeftSelectorShape(ctx), ctx.Config.TaskbarSlideActiveColor, 0)
	es.ShiftLeftActive = dec.Window
	if err != nil {
		log.Fatal(err)
		return nil
	}
	dec, err = CreateDecoration(ctx, RightSelectorShape(ctx), ctx.Config.TaskbarSlideInactiveColor, 0)
	es.ShiftRightInactive = dec.Window
	if err != nil {
		log.Fatal(err)
		return nil
	}
	dec, err = CreateDecoration(ctx, RightSelectorShape(ctx), ctx.Config.TaskbarSlideActiveColor, 0)
	es.ShiftRightActive = dec.Window
	if err != nil {
		log.Fatal(err)
		return nil
	}

	es.ShiftLeftInactive.Map()
	es.ShiftRightInactive.Map()

	err = es.AddSlideHooks(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return es
}

func (es *ElementScroller) MoveResize(ctx *Context) {
	leftShape := LeftSelectorShape(ctx)
	es.ShiftLeftInactive.MoveResize(leftShape.X, leftShape.Y, leftShape.W, leftShape.H)
	es.ShiftLeftActive.MoveResize(leftShape.X, leftShape.Y, leftShape.W, leftShape.H)

	rightShape := RightSelectorShape(ctx)
	es.ShiftRightInactive.MoveResize(rightShape.X, rightShape.Y, rightShape.W, rightShape.H)
	es.ShiftRightActive.MoveResize(rightShape.X, rightShape.Y, rightShape.W, rightShape.H)

	es.CanFit = CalcCanFit(ctx)
	es.UpdateMappings(ctx)
	es.ShiftAndActivate(ctx)
}

func (es *ElementScroller) Get(c *Container) *Element {
	e, _ := es.Elements[c]
	return e
}

func (es *ElementScroller) ForEach(f func(*Element, int)) {
	idx := 0
	for next := es.Front; next != nil; next = next.Next {
		f(next, idx)
		idx++
	}
}

func (es *ElementScroller) ShiftAndActivate(ctx *Context) {
	if len(es.Elements) < (es.StartingIdx + es.CanFit) {
		es.StartingIdx = ext.IMax(len(es.Elements)-es.CanFit, 0)
	}

	es.ForEach(func(e *Element, idx int) {
		active := ShouldActivate(idx, es.StartingIdx, es.CanFit)
		if !active && !e.Active {
			return
		}
		e.Active = active
		if e.Active {
			e.MoveResize(ctx, idx)
		}
		e.UpdateMapping(ctx)
	})

	es.UpdateMappings(ctx)
}

func (es *ElementScroller) UpdateMappings(ctx *Context) {
	if ctx.Taskbar.Hidden {
		es.ShiftLeftActive.Unmap()
		es.ShiftLeftInactive.Unmap()
		es.ShiftRightActive.Unmap()
		es.ShiftRightInactive.Unmap()
		es.ForEach(func(e *Element, idx int) {
			e.UpdateMapping(ctx)
		})
		return
	}
	if es.StartingIdx != 0 {
		es.ShiftLeftInactive.Unmap()
		es.ShiftLeftActive.Map()
	} else {
		es.ShiftLeftInactive.Map()
		es.ShiftLeftActive.Unmap()
	}
	if len(es.Elements) > (es.StartingIdx + es.CanFit) {
		es.ShiftRightInactive.Unmap()
		es.ShiftRightActive.Map()
	} else {
		es.ShiftRightInactive.Map()
		es.ShiftRightActive.Unmap()
	}
	es.ForEach(func(e *Element, idx int) {
		e.UpdateMapping(ctx)
	})
}

func (es *ElementScroller) Raise(ctx *Context) {
	es.ShiftLeftActive.Stack(xproto.StackModeAbove)
	es.ShiftLeftInactive.Stack(xproto.StackModeAbove)
	es.ShiftRightActive.Stack(xproto.StackModeAbove)
	es.ShiftRightInactive.Stack(xproto.StackModeAbove)
	es.ForEach(func(e *Element, idx int) {
		e.Window.Stack(xproto.StackModeAbove)
		e.MinWin.Stack(xproto.StackModeAbove)
	})
}

func (es *ElementScroller) Lower(ctx *Context) {
	es.ShiftLeftActive.Stack(xproto.StackModeBelow)
	es.ShiftLeftInactive.Stack(xproto.StackModeBelow)
	es.ShiftRightActive.Stack(xproto.StackModeBelow)
	es.ShiftRightInactive.Stack(xproto.StackModeBelow)
	es.ForEach(func(e *Element, idx int) {
		e.Window.Stack(xproto.StackModeBelow)
		e.MinWin.Stack(xproto.StackModeBelow)
	})
}

func NewElement(ctx *Context, c *Container, idx int) *Element {
	elem := &Element{
		Container: c,
	}

	win, err := xwindow.Generate(ctx.X)
	if err != nil {
		log.Println(err)
	}
	shape := ElementShape(ctx, idx)
	win.Create(ctx.X.RootWin(), shape.X, shape.Y, shape.W, shape.H, 0)
	elem.Window = win

	dec, err := CreateDecoration(ctx, MinWinShape(ctx, shape), ctx.Config.TaskbarMinMaxColor, 0)
	if err != nil {
		log.Println(err)
	}
	elem.MinWin = dec.Window
	elem.AddIconHooks(ctx)

	return elem
}

func (e *Element) Raise(ctx *Context) {
	e.Window.Stack(xproto.StackModeAbove)
	e.MinWin.Stack(xproto.StackModeAbove)
}

func (e *Element) Lower(ctx *Context) {
	e.Window.Stack(xproto.StackModeBelow)
	e.MinWin.Stack(xproto.StackModeBelow)
}

func (e *Element) UpdateMapping(ctx *Context) {
	if ctx.Taskbar.Hidden || !e.Active {
		e.MinWin.Unmap()
		e.Window.Unmap()
	} else {
		if e.Container.Hidden {
			e.MinWin.Map()
		} else {
			e.MinWin.Unmap()
		}
		e.Window.Map()
	}
}

func (e *Element) Unmap() {
	e.MinWin.Unmap()
	e.Window.Unmap()
}

func (e *Element) Destroy() {
	e.Window.Destroy()
	e.MinWin.Destroy()
}

func (e *Element) MoveResize(ctx *Context, idx int) {
	shape := ElementShape(ctx, idx-ctx.Taskbar.Scroller.StartingIdx)
	e.Window.MoveResize(shape.X, shape.Y, shape.W, shape.H)
	mwShape := MinWinShape(ctx, shape)
	e.MinWin.MoveResize(mwShape.X, mwShape.Y, mwShape.W, mwShape.H)
}

func (e *Element) AddIconHooks(ctx *Context) error {
	err := mousebind.ButtonPressFun(func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
		if ctx.Locked {
			return
		}

		if !e.Container.Hidden {
			f := ctx.GetFocusedFrame()
			if f == nil || f.Container != e.Container {
				e.Container.RaiseFindFocus(ctx)
				return
			}
		}

		e.Container.ChangeMinimizationState(ctx)
	}).Connect(ctx.X, e.Window.Id, ctx.Config.ButtonClick, false, true)
	if err != nil {
		return err
	}
	return err
}

func (es *ElementScroller) SlideLeft(ctx *Context) {
	if ctx.Locked {
		return
	}

	es.StartingIdx = ext.IMax(es.StartingIdx-1, 0)
	es.UpdateMappings(ctx)
	es.ShiftAndActivate(ctx)
}

func (es *ElementScroller) SlideRight(ctx *Context) {
	if ctx.Locked {
		return
	}

	if (es.StartingIdx + es.CanFit) > len(es.Elements) {
		return
	}
	es.StartingIdx = ext.IMin(es.StartingIdx+1, ext.IMax(len(es.Elements)-1, 0))
	es.UpdateMappings(ctx)
	es.ShiftAndActivate(ctx)
}

func (es *ElementScroller) AddSlideHooks(ctx *Context) error {
	err := mousebind.ButtonPressFun(func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
		es.SlideLeft(ctx)
	}).Connect(ctx.X, es.ShiftLeftActive.Id, ctx.Config.ButtonClick, false, true)
	if err != nil {
		return err
	}

	err = mousebind.ButtonPressFun(func(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
		es.SlideRight(ctx)
	}).Connect(ctx.X, es.ShiftRightActive.Id, ctx.Config.ButtonClick, false, true)
	if err != nil {
		return err
	}

	return err
}

func LeftSelectorShape(ctx *Context) Rect {
	tshape := TaskbarShape(ctx)
	return Rect{
		X: tshape.X,
		Y: tshape.Y,
		W: ctx.Config.TaskbarSlideWidth,
		H: ctx.Config.TaskbarHeight,
	}
}

func RightSelectorShape(ctx *Context) Rect {
	bs := BarrierElementShape(ctx)
	return Rect{
		X: bs.X - ctx.Config.TaskbarSlideWidth - ctx.Config.TaskbarXPad,
		Y: TaskbarShape(ctx).Y,
		W: ctx.Config.TaskbarSlideWidth,
		H: ctx.Config.TaskbarHeight,
	}
}

func MinWinShape(ctx *Context, elementShape Rect) Rect {
	return Rect{
		X: elementShape.X,
		Y: elementShape.Y + elementShape.H,
		W: elementShape.W,
		H: ctx.Config.TaskbarMinMaxHeight,
	}
}

func (t *Taskbar) MoveResize(ctx *Context) {
	s := TaskbarShape(ctx)
	t.Base.Window.MoveResize(s.X, s.Y, s.W, s.H)
	st := TimeShape(ctx)
	t.TimeWin.MoveResize(st.X, st.Y, st.W, st.H)
	sb := BatShape(ctx)
	t.BatWin.MoveResize(sb.X, sb.Y, sb.W, sb.H)
	t.Scroller.MoveResize(ctx)
}

func (t *Taskbar) Update(ctx *Context) {
	now := time.Now()
	text.DrawText(
		t.TimeWin,
		prompt.DefaultInputTheme.Font,
		ctx.Config.TaskbarFontSize,
		render.NewColor(int(ctx.Config.TaskbarTextColor)),
		render.NewColor(int(ctx.Config.TaskbarBaseColor)),
		now.Format(ctx.Config.TaskbarTimeFormat),
	)

	batteries, _ := battery.GetAll()
	bat, charging := func() (int, string) {
		if len(batteries) == 0 {
			return t.History.LastBattery, t.History.LastBatteryState
		}

		level := int(batteries[0].Current / batteries[0].Full * 100)
		state := "∘"
		if batteries[0].State == battery.Charging {
			state = "⇡"
		}

		for _, lvl := range ctx.Config.BatteryWarningLevels {
			if t.History.LastBattery > lvl && level <= lvl {
				msgPrompt := prompt.NewMessage(ctx.X, prompt.DefaultMessageTheme, prompt.DefaultMessageConfig)
				for _, screen:= range ctx.Screens {
				  msgPrompt.Show(screen.ToXRect(), fmt.Sprintf("Warning: battery at %d%%", level), ctx.Config.BatteryWarningDuration, func(msg *prompt.Message) {})
				}
				break
			}
		}

		t.History.LastBattery = level
		t.History.LastBatteryState = state
		return level, state
	}()

	text.DrawText(
		t.BatWin,
		prompt.DefaultInputTheme.Font,
		ctx.Config.TaskbarFontSize,
		render.NewColor(int(ctx.Config.TaskbarTextColor)),
		render.NewColor(int(ctx.Config.TaskbarBaseColor)),
		fmt.Sprintf(ctx.Config.TaskbarBatFormat, charging, bat),
	)
}

func (t *Taskbar) UpdateMapping(ctx *Context) {
	if t.Hidden {
		t.Base.Window.Unmap()
		t.TimeWin.Unmap()
		t.BatWin.Unmap()
	} else {
		t.Base.Window.Map()
		t.TimeWin.Map()
		t.BatWin.Map()
	}
	t.Scroller.UpdateMappings(ctx)
}

func (t *Taskbar) Raise(ctx *Context) {
	t.Base.Window.Stack(xproto.StackModeAbove)
	t.TimeWin.Stack(xproto.StackModeAbove)
	t.BatWin.Stack(xproto.StackModeAbove)
	t.Scroller.Raise(ctx)
}

func (t *Taskbar) Lower(ctx *Context) {
	t.Base.Window.Stack(xproto.StackModeBelow)
	t.TimeWin.Stack(xproto.StackModeBelow)
	t.BatWin.Stack(xproto.StackModeBelow)
	t.Scroller.Lower(ctx)
}

func CalcCanFit(ctx *Context) int {
	tshape := TaskbarShape(ctx)
	iconwidth := ctx.Config.TaskbarElementShape.X*2 + ctx.Config.TaskbarElementShape.W
	selectorwidth := ctx.Config.TaskbarSlideWidth
	bs := BarrierElementShape(ctx)
	rightpad := tshape.W - (bs.X - tshape.X) - ctx.Config.TaskbarXPad
	return (tshape.W - selectorwidth*2 - rightpad) / iconwidth
}

func ElementShape(ctx *Context, indexOffset int) Rect {
	tshape := TaskbarShape(ctx)
	startX := tshape.X
	startX = startX + ctx.Config.TaskbarSlideWidth
	increment := ctx.Config.TaskbarElementShape.X*2 + ctx.Config.TaskbarElementShape.W
	startX = startX + increment*indexOffset
	return Rect{
		X: startX + ctx.Config.TaskbarElementShape.X,
		Y: tshape.Y + ctx.Config.TaskbarElementShape.Y,
		W: ctx.Config.TaskbarElementShape.W,
		H: ctx.Config.TaskbarElementShape.H,
	}
}

func TaskbarShape(ctx *Context) Rect {
	if len(ctx.Screens) == 0 {
		return Rect{
			X: 0,
			Y: 0,
			W: ctx.Config.ElemSize,
			H: ctx.Config.ElemSize,
		}
	}
	return Rect{
		X: ctx.Screens[0].X,
		Y: ctx.Screens[0].Y + ctx.Screens[0].H - ctx.Config.TaskbarHeight,
		W: ctx.Screens[0].W,
		H: ctx.Config.TaskbarHeight,
	}
}

func BarrierElementShape(ctx *Context) Rect {
	return TimeShape(ctx)
}

func TimeShape(ctx *Context) Rect {
	ew, eh := xgraphics.Extents(prompt.DefaultInputTheme.Font, ctx.Config.TaskbarFontSize, ctx.Config.TaskbarTimeFormat)
	s := BatShape(ctx)
	return Rect{
		X: s.X - ew - 2*ctx.Config.TaskbarXPad,
		Y: s.Y,
		W: ew,
		H: eh,
	}
}

func BatShape(ctx *Context) Rect {
	ew, eh := xgraphics.Extents(prompt.DefaultInputTheme.Font, ctx.Config.TaskbarFontSize, fmt.Sprintf(ctx.Config.TaskbarBatFormat, "∘", 50))
	s := TaskbarShape(ctx)
	return Rect{
		X: s.X + s.W - ew - ctx.Config.TaskbarXPad,
		Y: s.Y + s.H - eh - ctx.Config.TaskbarYPad,
		W: ew,
		H: eh,
	}
}

package workbench

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// Exercise the real composer/provider/placement path. Windows' preservation
// of the native IME session across these caret updates is covered in Viva.
func TestAIComposerPreeditCommitAndSend(t *testing.T) {
	state, _ := panelState(t, quietModel{answer: "收到"})
	withModel(t, state)
	state.Click(hitAIToggle)
	paintAt(state, panelWide)
	apply := func(event app.TextEvent) app.TextState {
		t.Helper()
		input := state.AIPanelInput(app.TextInputContext{ID: hitAIInput})
		if input == nil || input.OnEvent == nil {
			t.Fatal("AI composer has no native text handler")
		}
		next, err := input.OnEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		canvas, _ := paintAt(state, panelWide)
		hit := panelHit(t, canvas, hitAIInput)
		if !hit.Multiline || hit.Placement == nil {
			t.Fatal("AI composer lost its multiline caret placement")
		}
		if _, ok := hit.Placement.CaretBounds(); !ok {
			t.Fatal("AI composer has no candidate-window anchor")
		}
		if got := state.AIPanelInput(app.TextInputContext{ID: hitAIInput}).State; got != app.TextStateOf(&state.ai.input) {
			t.Fatalf("provider mirrors stale state: %+v", got)
		}
		return next
	}
	for _, prefix := range []string{"", "你好\n"} {
		for _, preedit := range []string{"n", "ni", "nih", "nihao"} {
			next := apply(app.TextEvent{Kind: app.TextSetMarked, Text: preedit, MarkedSelection: app.TextRange{Start: len(preedit), End: len(preedit)}})
			start := len([]rune(prefix))
			if next.Value != prefix+preedit || !next.Composing || next.Composition != (app.TextRange{Start: start, End: start + len(preedit)}) {
				t.Fatalf("preedit %q = %+v", preedit, next)
			}
			if state.ai.run != nil || len(state.ai.items) != 0 || state.ai.input.String() != prefix+preedit {
				t.Fatal("preedit changed or sent the message")
			}
		}
		next := apply(app.TextEvent{Kind: app.TextInsert, Text: "你好"})
		if next.Value != prefix+"你好" || next.Composing {
			t.Fatalf("commit = %+v", next)
		}
		if prefix == "" {
			apply(app.TextEvent{Kind: app.TextCommandEvent, Command: app.TextInsertLineBreak})
		}
	}
	apply(app.TextEvent{Kind: app.TextCommandEvent, Command: app.TextInsertNewline})
	if state.ai.input.String() != "" || state.ai.run == nil && len(state.ai.items) == 0 {
		t.Fatal("Enter did not send the committed message")
	}
	pumpWindow(t, state, func() bool { return state.ai.run == nil })
}

func TestAIComposerModifiedDeleteCommands(t *testing.T) {
	state, _ := panelState(t, nil)
	state.Click(hitAIToggle)
	canvas, _ := paintAt(state, panelWide)
	box := panelHit(t, canvas, hitAIInput).Bounds
	var nativeInput app.TextInput
	driver, err := app.NewDriver(app.Config{
		Title: "assistant modified delete", Width: int(panelWide.Width), Height: int(panelWide.Height), Tree: Tree(),
		Paint: state.Paint,
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if input := state.AIPanelInput(context); input != nil {
				return input
			}
			editor := state.EditorFor(context.ID)
			if editor == nil {
				return nil
			}
			nativeInput = app.TextInput{Editor: editor}
			return &nativeInput
		},
		OnPointer: state.Pointer, OnKey: state.Key, OnShortcut: state.Shortcut,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(box.X+box.Width/2, box.Y+box.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("第一行\n第二行"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyBackspace, ui.ModifierMeta); err != nil {
		t.Fatal(err)
	}
	if got := state.ai.input.String(); got != "第一行\n" {
		t.Fatalf("Cmd+Backspace left %q, want the second line removed", got)
	}
	if err := driver.SelectText(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyDelete, ui.ModifierMeta); err != nil {
		t.Fatal(err)
	}
	if got := state.ai.input.String(); got != "\n" {
		t.Fatalf("Cmd+Delete left %q, want the first line removed", got)
	}
}

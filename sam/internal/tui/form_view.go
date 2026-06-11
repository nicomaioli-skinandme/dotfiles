package tui

import (
	"fmt"
	"strings"
)

// renderForm draws the add-workspace form into the body area: a header,
// the answered steps as compact ✓ lines, then the active field (or the
// busy spinner / failure banner) and a key-hint footer. When the form
// outgrows the body, the oldest answered lines are dropped first.
func (m *model) renderForm(h int) string {
	f := m.form
	st := m.styles

	header := []string{"  " + st.breadcrumb.Render("Add workspace")}
	if f.firstRun {
		header = append(header, "  "+st.hint.Render("first-time setup — configure a workspace"))
	}
	header = append(header, "")

	var answered []string
	for i := range f.steps {
		s := &f.steps[i]
		if s.summary == "" {
			continue
		}
		answered = append(answered, fmt.Sprintf("  %s %s  %s",
			st.selected.Render("✓"), st.row.Render(s.title), st.detail.Render(s.summary)))
	}

	var body []string
	switch {
	case f.busy != "":
		body = append(body, "  "+m.spinner.View()+" "+st.hint.Render(f.busy))

	case f.failure != "":
		body = append(body,
			"  "+st.deleting.Render(oneLine(f.failure)),
			"",
			"  "+st.hint.Render("r retry · esc back"))

	default:
		s := f.active()
		body = append(body, "  "+st.cursor.Render(s.title))
		if s.desc != "" {
			body = append(body, "  "+st.hint.Render(s.desc))
		}
		body = append(body, m.renderFormControl(s)...)
		if s.errMsg != "" {
			body = append(body, "  "+st.deleting.Render(oneLine(s.errMsg)))
		}
		body = append(body, "", "  "+st.hint.Render(m.formFooter(s)))
	}

	// Answered lines yield first when space runs out: the active field and
	// footer must stay visible.
	for len(header)+len(answered)+1+len(body) > h && len(answered) > 0 {
		answered = answered[1:]
	}

	lines := make([]string, 0, len(header)+len(answered)+1+len(body))
	lines = append(lines, header...)
	lines = append(lines, answered...)
	lines = append(lines, "")
	lines = append(lines, body...)
	for i := range lines {
		lines[i] = truncate(lines[i], m.width)
	}
	return pad(strings.Join(lines, "\n"), m.width, h)
}

// renderFormControl draws the active step's input control.
func (m *model) renderFormControl(s *formStep) []string {
	st := m.styles
	switch s.kind {
	case fieldInput:
		return []string{"  " + s.input.View()}

	case fieldConfirm:
		no := st.modalAffirm.Render("No")
		yes := st.modalAffirm.Render("Yes")
		if s.yes {
			yes = st.modalActive.Render("Yes")
		} else {
			no = st.modalActive.Render("No")
		}
		return []string{"  " + no + "   " + yes}

	case fieldSelect, fieldMulti:
		lines := make([]string, 0, len(s.options))
		for i, o := range s.options {
			cursor := "  "
			if i == s.cursor {
				cursor = st.cursor.Render("▸ ")
			}
			check := ""
			if s.kind == fieldMulti {
				check = "  "
				if s.checked[o.value] {
					check = st.selected.Render("✓") + " "
				}
			}
			label := o.label
			if i == s.cursor {
				label = st.cursor.Render(label)
			} else {
				label = st.row.Render(label)
			}
			lines = append(lines, "  "+cursor+check+label)
		}
		return lines
	}
	return nil
}

// formFooter is the key hint under the active step.
func (m *model) formFooter(s *formStep) string {
	back := "esc back"
	if len(m.form.steps) == 1 {
		back = "esc cancel"
		if m.form.firstRun {
			back = "esc quit"
		}
	}
	if s.kind == fieldMulti {
		return "space toggle · enter next · " + back
	}
	return "enter next · " + back
}

package attach

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/creydr/ai-mux/internal/provider"
)

func renderHeader(item *provider.Item) string {
	if item == nil {
		return headerStyle.Render("Loading...")
	}
	prefix := "#"
	if item.Type == provider.ItemTypePR {
		prefix = "PR #"
	}
	title := fmt.Sprintf("%s%d: %s", prefix, item.Number, item.Title)
	meta := fmt.Sprintf("  %s  by %s  [%s]",
		labelStyle.Render(item.Repo.String()),
		item.Author,
		item.State,
	)
	return headerStyle.Render(title) + "\n" + meta
}

func renderBody(item *provider.Item, width int) string {
	if item == nil || item.Body == "" {
		return bodyStyle.Render("No description")
	}
	return renderMarkdown(item.Body, width)
}

func renderMarkdown(text string, width int) string {
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return bodyStyle.Render(text)
	}
	rendered, err := r.Render(text)
	if err != nil {
		return bodyStyle.Render(text)
	}
	return strings.TrimRight(rendered, "\n")
}

func renderReviews(reviews []provider.Review) string {
	if len(reviews) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("  Reviews"))
	b.WriteString("\n")
	for _, r := range reviews {
		b.WriteString(reviewStyle.Render(
			reviewAuthorStyle.Render(r.Author) + " " +
				reviewStateStyle.Render(r.State),
		))
		b.WriteString("\n")
		if r.Body != "" {
			b.WriteString(bodyStyle.Render("    " + r.Body))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderComments(comments []provider.Comment) string {
	if len(comments) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("  Comments"))
	b.WriteString("\n")
	for _, c := range comments {
		b.WriteString(commentStyle.Render(
			commentAuthorStyle.Render(c.Author) + ": " + c.Body,
		))
		b.WriteString("\n")
	}
	return b.String()
}

// Package fix builds the text edits for a planned rewrite: template
// expansion with comment placement (ExpandNotes) and import changes. Argument
// text is always copied verbatim from the source; nothing is re-printed
// through go/printer.
package fix

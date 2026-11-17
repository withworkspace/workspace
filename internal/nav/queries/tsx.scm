; TSX's own, after TypeScript's: a component named in JSX is called; an
; element of the page (<div>) isn't a name.

(jsx_opening_element name: (identifier) @ref.call)
(jsx_self_closing_element name: (identifier) @ref.call)
(jsx_closing_element name: (identifier) @ref.call)
(jsx_closing_element name: (identifier) @skip (#match? @skip "^[a-z]"))
(jsx_opening_element name: (identifier) @skip (#match? @skip "^[a-z]"))
(jsx_self_closing_element name: (identifier) @skip (#match? @skip "^[a-z]"))

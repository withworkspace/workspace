Token cache
===========

A paragraph with *emphasis*, __strong__, ~~struck~~, `code`, a [link](https://example.com "Example"),
a soft break, and a bare https://example.com/path link, www.example.org and team@example.com.
A hard break follows\
and this is after it. An image: ![a diagram](diagram.png). Escaped \*stars\* and &amp; entities.

## Lists

* A star list
* With a [reference link][ref] and a footnote[^1]
    * nested four deep
    * another

1) Paren numbers
2) Count up

1. Same number
1. Every time
1. Like this

- [ ] A task
- [x] Done task
- [ ] Task with a nested list
  - first
  - second

- Loose list

- With blank lines

  And a second paragraph in an item.

10. Starts at ten
11. Then eleven

## Quotes

> A quote that runs
> over two lines.
>
> - a list in a quote
> - another
>
> > nested quote

## Code

```go title="main.go"
func main() {}
```

~~~
tilde fenced
~~~

    indented code

## Tables

| Kind | What you want | What the agent does |
|---|:---:|--:|
| **Ask** | An answer. | Answers in the thread. |
| Change | The document changed. | Suggests an edit. |
| Note | To say something. | Nothing. |

| a   | b   |
| --- | --- |
| one | two |

## Raw

<details>
<summary>Raw HTML</summary>

Inside it.

</details>

Some <kbd>inline</kbd> HTML.

***

Text after the rule.

[ref]: https://example.com/ref "Ref"
[^1]: The footnote,
    over two lines.

# KING OF LUNCH

A quiet place to read Markdown. This document exercises the complete reading surface: **strong text**, *emphasis*, ~~strikethrough~~, and `inline code`. Unicode stays intact: 日本語 · café · 🍱.

[Jump to the wide table](#wide-table) · [Open another Markdown file](linked%20note.md) · [Visit the Go website](https://go.dev/)

## A small menu

- Rice and vegetables
  - Pickled cucumber
  - Miso dressing
- Soup of the day

1. Open a local Markdown file.
2. Make a change in your editor.
3. Press **⌘R** to reload it here.

- [x] Read a document
- [x] Keep the interface out of the way
- [ ] Decide what is for lunch

> The document is the focal point. Native menus, comfortable typography, and the Kanagawa Wave palette carry the rest.
>
> A second paragraph in the same quotation.

## Syntax and whitespace

```go
package main

import "fmt"

func main() {
	meal := "rice and vegetables"
	fmt.Printf("Today's lunch: %s — table %d\n", meal, 42)
}
```

```python
def lunch(people: list[str]) -> dict[str, bool]:
    # A comment should remain comfortably legible.
    return {person: True for person in people}

print(lunch(["Ada", "Grace", "Linus"]))
```

```json
{"app": "KING OF LUNCH", "readOnly": true, "fontSize": 16}
```

```unknown-language
Unknown languages stay plain, including <escaped markup> and & symbols.
```

    Indented code works as well.
    Tabs and spaces remain part of the document.

## Wide table

| Dish | Origin | Notes | Price | Extra-long field to exercise horizontal scrolling | Another wide field |
| :--- | :---: | :--- | ---: | :--- | :--- |
| Vegetable curry | Japan | Sweet, warm, and filling | 12.50 | This deliberately wide table should stay reachable even in a narrow window. | A second long description keeps the table wider than the reading measure. |
| Noodle soup | Vietnam | Fresh herbs and lime | 14.00 | Tables preserve alignment and their own horizontal scrolling region. | Use a trackpad or focus the region and press the arrow keys. |

```bash
kol '/a/deliberately/long/path/with many spaces/and-more-components/that/keeps/going/so/this/code/block/needs/horizontal/scrolling/document.md'
```

## Images and boundaries

![Local Kanagawa color swatch](kanagawa.png "An image in the document folder")

![Missing image keeps this useful alternative text](not-present.png)

![Remote images remain offline](https://example.com/tracker.png)

Unsafe links preserve their labels: [a blocked script](javascript:alert%281%29). Raw HTML is omitted:

<script>document.body.textContent = 'THIS MUST NEVER APPEAR';</script>

---

### Smaller heading

Long unbroken text should remain reachable and prose should wrap: abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.

#### Fourth level

An ordinary paragraph follows. Select it and use **⌘C** to copy.

##### Fifth level

Text at every heading level remains proportional during font zoom.

###### Sixth level

The end of the fixture. **⌘+**, **⌘−**, and **⌘0** change and reset the font size.

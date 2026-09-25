# Assets - Overview

What the `view` package embeds into the binary and inlines into the pages it
renders.

| file                 | goes out as                                     |
|----------------------|-------------------------------------------------|
| `knut.html`          | every page: the layout and one block per page    |
| `knut.css`           | inlined into every page                          |
| `listing-filter.js`  | inlined into every listing                       |
| `listing-keys.js`    | inlined into every listing                       |
| `listing-qr.js`      | inlined into a listing which offers codes        |
| `htmx.min.js.gz`     | served at a reserved uri, `-live` only           |

Every script is an addition: a listing without them is read, sorted and
clicked like any other page.


## listing-filter.js

A box in front of the listing which narrows it while it is typed in. The
entries are already on screen, so the server is not involved and no flag is
needed.

- terms are separated by spaces and all have to match: `log 2026`
- `-term` excludes: `log -old` is every name carrying `log`, minus the ones
  carrying `old`
- matching is case insensitive and looks at the name column only
- `/` focuses the box, `Escape` empties it
- the query is kept in `sessionStorage` under the path, so a click on a
  column header - a plain link, which reloads the page - does not drop it
- the `../` row carries `class="parent"` and is never hidden
- without javascript, and in a folder with no rows, there is no box:
  `knut.html` renders it `hidden` and `apply()` unhides it while a table is
  on the page

### Markup

`knut.html` renders the box and the table as two blocks, one after the
other:

```html
<div class="filter">          <!-- the box and its count -->
  <input id="filter"> <span id="filter-count"></span>
</div>
<div id="listing">            <!-- what a live listing replaces -->
  <table class="listing"> ... </table>
</div>
```

`#listing` is the block a live listing refreshes: one request which the
server holds open until the folder changes (`listing.watch()` in listing.go,
`watchListing()` in listing_watch.go), answered with that block alone and
swapped in whole.

The box is kept out of it on purpose - inside, a swap would take the typed
query and the caret with it. `apply()` re-runs on `htmx:afterSwap` for the
other half of the same reason: the rows which arrive are unfiltered.

### Matching

Each keystroke matches exactly, with `indexOf` over the name of each row.
Where that finds nothing at all the term is taken for a typo and the pass is
repeated with `within()`, which allows one edit per term; the count then
reads `no match ι N near` instead of `N of M`.

One edit, never two, and none below four characters - `nearly()` and
`excluded()` also keep `-term` exclusions exact, a loose one would hide rows
that were asked for. A single edit on a short term already matches a large
part of a folder, and an edit over a digit matches every other digit, so
`backup-12` widens further than a word of that length. That is why the loose
pass only runs when the exact one came back empty. What it buys is the
ordinary typo: `reprot`, `reort`, `repotr`, `rpeort` all reach `report`.

`within()` fills the edit distance table row by row, starting from a row of
zeros so the term may begin anywhere in the name, and gives up as soon as
every window is further away than one edit. The fourth branch of the inner
loop charges one edit for two neighbours swapped, which is what carries
`reprot`. That is Sellers' algorithm over the Damerau-Levenshtein metric in
its optimal string alignment form, with Ukkonen's cutoff. A bit-parallel
bitap would be quicker and is not worth the code: the pass only ever runs on
a term which matched nothing.


## listing-keys.js

Walks the rows of a listing from the keyboard. Which key does what is the
`keydown` handler in the file itself.

- the bar is a `selected` class on a `<tr>`, drawn as a solid block of the
  accent colour. a pointer passing over a row only tints it, so the two do
  not read alike
- it walks the rows on screen: a row the filter hid is skipped, and the ends
  of the table are ends, not a wrap-around
- keys are dropped while something is typed into an input and while a dialog
  is open - a code on screen has the keyboard, Escape included
- `Enter` is taken only while nothing is focused (`event.target` is the
  body), so a link which already has the focus is not followed twice

The bar survives both things which rewrite the rows under it. The name of
its row is kept in a variable and `restore()` looks that name up again after
the filter runs and after a live listing swaps `#listing`; a row which is
gone, or which the filter just hid, takes the bar with it. This script is
inlined after the filter so that its `htmx:afterSwap` listener is the second
one called - the bar is placed once the rows which arrived are hidden or
shown.


## listing-qr.js

The last column of a listing holds what can be done with a row: the `zip` of
a folder and the `qr` of either. Its header holds the same two for the
folder being listed, `?zip` and `./`.

`ListOpts` in listing.go says which of them a handler answers - a tree on
disk both, a listing of a zip neither - and `offer()` puts the links on the
rows and on the folder from that, so the markup never links what nobody
answers.

The `qr` link points at the entry itself, the uri the name column links to:
that is what the code carries. The png is a second resource, `<entry>?qr`,
drawn by the server when it is asked for, so a listing of a thousand rows
carries no codes at all.

The script turns the link into a dialog on top of the listing:

```html
<dialog id="qr-modal">
  <img id="qr-image"> <p id="qr-caption"></p>
</dialog>
```

- `showModal()` puts it in the top layer, `inset: 0` with `margin: auto`
  centers it there on both axes
- Escape closes it, and so does a click which lands on the dialog rather
  than on the image - as far as an event is concerned the backdrop is the
  dialog itself
- the caption is the url the code carries, the link resolved against the
  page, and the image is that same url plus `?qr`

Without the script the link is followed, which fetches the entry: a second
way to the file, not a code.

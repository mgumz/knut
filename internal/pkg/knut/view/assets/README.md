# Assets - Overview

What the `view` package embeds into the binary and inlines into the pages it
renders.

| file                 | goes out as                                     |
|----------------------|-------------------------------------------------|
| `knut.html`          | every page: the layout and one block per page    |
| `knut.css`           | inlined into every page                          |
| `listing-filter.js`  | inlined into every listing                       |
| `htmx.min.js.gz`     | served at a reserved uri, `-live` only           |


## listing-filter.js

In case of lengthy directory listings, a user might want to reduce the amount
of shown files. `listing-filter.js` implements a small "filter" box in front
of the directory listing in which the user can type in the files of interest.

The server is not involved: the entries are already on screen, and the
fingerprint a live listing polls with stays what the server rendered. It does
not need `-live`.

### UI

- terms are separated by spaces and all have to match: `log 2026`
- `/` focuses the box, `Escape` empties it
- matching is case insensitive and looks at the name column only
- `-term` excludes: `log -old` is every name carrying `log`, minus the ones
  carrying `old`
- the query is kept in `sessionStorage` under the path, so a click on a
  column header (a plain link, which reloads the page) does not drop it
- the `../` row carries `class="parent"`, which the filter skips: the way out
  of a folder is never hidden
- without javascript there is no box at all. `knut.html` renders it `hidden`
  and the script unhides it
- a folder with no rows has no box either. `apply()` decides that from
  whether a table is on the page, so a folder which falls empty while it is
  open loses its box, and one which fills up gets it back

### Markup

`knut.html` renders the box and the table as two blocks, one after the other:

```html
<div class="filter">          <!-- the box and its count -->
  <input id="filter"> <span id="filter-count"></span>
</div>
<div id="listing">            <!-- what a live listing replaces -->
  <table class="listing"> ... </table>
</div>
```

`#listing` is the block a live listing refreshes: the server re-renders it
and htmx swaps the new one in whole (`hx-select="#listing"`,
`listing.watch()` in listing.go). Each refresh is one request which the
server holds open until the folder changes - a long poll, see
`watchListing()` in listing_watch.go - and the block which comes back starts
the next one.

The box is kept out of that block on purpose. Inside it, it would be thrown
away with every swap, taking the typed query and the caret with it. `apply()`
re-runs on `htmx:afterSwap` for the other half of the same reason: the rows
which arrive are unfiltered.

The response carries the listing block alone: no `<!doctype html>`, no head,
header or footer, because the layout is already on screen.
`isFragment()` in page.go decides that from the `HX-Request` header htmx
sets. The browser parses such a response into a [`DocumentFragment`][df], a
node which holds a piece of a document without being one, and htmx moves its
children into the page - hence the name. Without `-live` it never happens:
every request is answered with a full page, whoever sent it.

[df]: https://developer.mozilla.org/en-US/docs/Web/API/DocumentFragment

### Matching

Each keystroke first tries to match exactly, with `indexOf` over the name of
each row. If that finds nothing at all, the term is assumed to carry a typo
and the match is retried with `within()`, which allows one edit per term. The
count then reads `no match ι N near` instead of `N of M`.

Two further limits, both in `nearly()` and `excluded()`: terms of three
characters or less are matched exactly even in the second pass, and `-term`
exclusions are always exact. A loose exclusion would hide rows that were
asked for.

One edit is allowed rather than two, and none at all below four characters. A
single edit on a short term already matches a large part of any folder, and
two edits on one match nearly all of it. Length alone is no guarantee either:
an edit over a digit matches every other digit, so a term like `backup-12`
widens much further than a word of the same length. That is the reason the
second pass runs only when the first one found nothing.

What it buys is the ordinary typo. Each of these is one edit away from
`report` - two letters swapped, one dropped, one doubled - so the second pass
finds the file where an exact match returns nothing at all:

```
reprot  reort  repotr  rpeort  reprort  reporrt
```

### The algorithm in `within()`

Sellers' algorithm: the Wagner-Fischer edit distance with its first row left
at zero - `prev` starts as a row of zeros - which lets the term begin
anywhere and makes this an approximate substring match rather than a
whole-name one (the *k*-differences problem, Sellers 1980).

The fourth branch of the inner loop, `prev2[j - 2] + 1`, charges one edit for
two neighbours swapped, and is what carries `reprot` to `report`. It makes
the metric Damerau-Levenshtein in its optimal string alignment form, the
restricted variant which never edits the same stretch twice; true
Damerau-Levenshtein needs a last-occurrence table for a distinction no typo
makes.

`if (best > k) return false` is Ukkonen's cutoff: once every window is
further than `k` away, no remaining column can bring one back.

A bit-parallel bitap (Baeza-Yates-Gonnet, extended to k errors by Wu-Manber,
as in `agrep`) is the usual alternative. It is not used here: the second pass
only ever runs on a term which matched nothing, and the loop above can be
checked by reading.

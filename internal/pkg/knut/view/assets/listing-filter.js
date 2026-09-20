// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

(function () {
	"use strict";

	var box = document.getElementById("filter");
	if (!box) {
		return;
	}

	var count = document.getElementById("filter-count");
	var store = "knut:filter:" + location.pathname;

	// sellers, damerau-levenshtein in its optimal string alignment form
	function within(name, term, k) {
		var n = name.length,
			m = term.length,
			prev2 = new Array(n + 1).fill(0),
			prev = new Array(n + 1).fill(0),
			cur = new Array(n + 1).fill(0),
			i, j, v, best, swap;

		for (i = 1; i <= m; i++) {
			cur[0] = best = i;
			for (j = 1; j <= n; j++) {
				v = prev[j - 1] + (term[i - 1] === name[j - 1] ? 0 : 1);
				if (prev[j] + 1 < v) {
					v = prev[j] + 1;
				}
				if (cur[j - 1] + 1 < v) {
					v = cur[j - 1] + 1;
				}
				// transposition
				if (i > 1 && j > 1 && term[i - 1] === name[j - 2] &&
					term[i - 2] === name[j - 1] && prev2[j - 2] + 1 < v) {
					v = prev2[j - 2] + 1;
				}
				cur[j] = v;
				if (v < best) {
					best = v;
				}
			}
			if (best > k) {
				return false;
			}
			swap = prev2;
			prev2 = prev;
			prev = cur;
			cur = swap;
		}

		return prev.some(function (d) {
			return d <= k;
		});
	}

	function query(text) {
		var want = [],
			not = [];

		text.toLowerCase().split(/\s+/).forEach(function (term) {
			if (term.length > 1 && term[0] === "-") {
				not.push(term.slice(1));
			} else if (term && term !== "-") {
				want.push(term);
			}
		});

		return { want: want, not: not, empty: !want.length && !not.length };
	}

	function excluded(name, q) {
		return q.not.some(function (term) {
			return name.indexOf(term) >= 0;
		});
	}

	function exact(name, q) {
		return !excluded(name, q) && q.want.every(function (term) {
			return name.indexOf(term) >= 0;
		});
	}

	function nearly(name, q) {
		return !excluded(name, q) && q.want.every(function (term) {
			return name.indexOf(term) >= 0 ||
				(term.length > 3 && within(name, term, 1));
		});
	}

	function show(rows, q, match) {
		var shown = 0;

		rows.forEach(function (row) {
			var hit = match(row.name, q);
			row.tr.hidden = !hit;
			if (hit) {
				shown++;
			}
		});

		return shown;
	}

	function rowsOf(table) {
		return Array.prototype.map.call(
			table.querySelectorAll("tbody tr:not(.parent)"),
			function (tr) {
				return { tr: tr, name: tr.querySelector(".name").textContent.toLowerCase() };
			}
		);
	}

	function apply() {
		var table = document.querySelector("table.listing"),
			rows, q, hits, fuzzy;

		box.parentElement.hidden = !table;
		if (!table) {
			return;
		}

		rows = rowsOf(table);
		q = query(box.value);
		hits = show(rows, q, exact);

		fuzzy = false;
		if (hits === 0 && q.want.length) {
			hits = show(rows, q, nearly);
			fuzzy = hits > 0;
		}

		if (count) {
			count.textContent = q.empty ? "" :
				fuzzy ? "no match ι " + hits + " near" :
				hits + " of " + rows.length;
		}
	}

	function remember() {
		try {
			sessionStorage.setItem(store, box.value);
		} catch (e) {
			// a browser refusing the store just forgets the query
		}
	}

	function recall() {
		try {
			return sessionStorage.getItem(store) || "";
		} catch (e) {
			return "";
		}
	}

	box.addEventListener("input", function () {
		apply();
		remember();
	});

	document.addEventListener("keydown", function (event) {
		var typing = event.target.tagName === "INPUT" ||
			event.target.tagName === "TEXTAREA";

		if (event.key === "/" && !typing && !event.ctrlKey && !event.metaKey && !event.altKey) {
			event.preventDefault();
			box.focus();
			box.select();
		} else if (event.key === "Escape" && event.target === box) {
			box.value = "";
			apply();
			remember();
			box.blur();
		}
	});

	document.body.addEventListener("htmx:afterSwap", apply);

	box.value = recall();
	box.hidden = false;
	apply();
})();

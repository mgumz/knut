// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

(function () {
	"use strict";

	// the name on the bar, kept across a live refresh - the rows which
	// come back are new nodes, the name is the same
	var selected = null;

	function rows() {
		return Array.prototype.filter.call(
			document.querySelectorAll("table.listing tbody tr"),
			function (tr) {
				return !tr.hidden;
			}
		);
	}

	function nameOf(tr) {
		var cell = tr.querySelector(".name");
		return cell ? cell.textContent : "";
	}

	function current() {
		return document.querySelector("table.listing tbody tr.selected");
	}

	function mark(tr) {
		var was = current();

		if (was) {
			was.classList.remove("selected");
		}

		selected = tr ? nameOf(tr) : null;
		if (tr) {
			tr.classList.add("selected");
			tr.scrollIntoView({ block: "nearest" });
		}
	}

	function at(open) {
		for (var i = 0; i < open.length; i++) {
			if (open[i].classList.contains("selected")) {
				return i;
			}
		}
		return -1;
	}

	// the bar starts at the end it is moving away from and stops at the
	// other one: a listing is a list, not a carousel
	function move(step) {
		var open = rows(), i;

		if (!open.length) {
			return;
		}

		i = at(open);
		if (i < 0) {
			i = step > 0 ? 0 : open.length - 1;
		} else {
			i = Math.min(Math.max(i + step, 0), open.length - 1);
		}

		mark(open[i]);
	}

	function follow() {
		var tr = current(),
			link = tr && tr.querySelector(".name a");

		if (link) {
			link.click();
		}
	}

	// a row which was filtered away or which is simply gone takes the bar
	// with it, the one still there keeps it
	function restore() {
		var open = rows(), i;

		if (selected === null) {
			return;
		}

		for (i = 0; i < open.length; i++) {
			if (nameOf(open[i]) === selected) {
				mark(open[i]);
				return;
			}
		}

		mark(null);
	}

	document.addEventListener("keydown", function (event) {
		var typing = event.target.tagName === "INPUT" ||
			event.target.tagName === "TEXTAREA";

		// a dialog on top of the listing has the keyboard
		if (typing || document.querySelector("dialog[open]")) {
			return;
		}
		if (event.ctrlKey || event.metaKey || event.altKey) {
			return;
		}

		if (event.key === "j" || event.key === "ArrowDown") {
			event.preventDefault();
			move(1);
		} else if (event.key === "k" || event.key === "ArrowUp") {
			event.preventDefault();
			move(-1);
		} else if (event.key === "Enter" && event.target === document.body) {
			// nothing is focused, so no link is about to be followed
			// twice
			event.preventDefault();
			follow();
		} else if (event.key === "Backspace") {
			event.preventDefault();
			history.back();
		} else if (event.key === "?" && help && help.showModal) {
			event.preventDefault();
			help.showModal();
		}
	});

	var help = document.getElementById("keys-modal");
	if (help) {
		help.addEventListener("click", function (event) {
			if (event.target === help) {
				help.close();
			}
		});
	}

	var box = document.getElementById("filter");
	if (box) {
		box.addEventListener("input", restore);
	}

	document.body.addEventListener("htmx:afterSwap", restore);
})();

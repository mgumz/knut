// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

(function () {
	"use strict";

	var box = document.getElementById("upload");
	var form = box && box.querySelector("form");
	if (!form || !window.FormData || !window.XMLHttpRequest) {
		return;
	}

	var input = form.querySelector('input[type="file"]');
	var list = document.getElementById("upload-files");
	var summary = document.getElementById("upload-summary");
	var more = document.getElementById("upload-more");

	var items = []; // every file picked, in the order picked
	var queue = []; // the ones still to send, a retry goes to the end
	var busy = false;
	var started = 0;

	// the same as humanSize() in page.go
	function humanSize(n) {
		var unit = 1024;
		if (n < unit) {
			return n + " B";
		}
		var units = ["KiB", "MiB", "GiB", "TiB", "PiB", "EiB"];
		var div = unit, exp = 0;
		for (var m = Math.floor(n / unit); m >= unit && exp < 5; m = Math.floor(m / unit)) {
			div *= unit;
			exp++;
		}
		return (n / div).toFixed(1) + " " + units[exp];
	}

	function el(tag, cls, text) {
		var node = document.createElement(tag);
		if (cls) {
			node.className = cls;
		}
		if (text !== undefined) {
			node.textContent = text;
		}
		return node;
	}

	function add(file) {
		var item = {
			file: file,
			li: el("li"),
			bar: el("progress"),
			note: el("span", "meta", "waiting"),
			ok: false
		};
		item.bar.max = 100;
		item.bar.value = 0;
		item.li.appendChild(el("span", "name", file.name));
		item.li.appendChild(item.bar);
		item.li.appendChild(item.note);
		list.appendChild(item.li);
		return item;
	}

	// the bar is honest about what it measures: it fills while the
	// browser sends, so it stands at 100% while the last of the file is
	// still being written to disk - that is what the note says then
	function send(item, done) {
		var data = new FormData();
		data.append(input.name, item.file, item.file.name);

		var xhr = new XMLHttpRequest();
		xhr.open("POST", form.action);
		// the text form of the answer: the code is what counts, the
		// rest of it is not shown
		xhr.setRequestHeader("Accept", "text/plain");
		xhr.upload.onprogress = function (event) {
			if (!event.lengthComputable) {
				return;
			}
			item.bar.value = event.loaded / event.total * 100;
			item.note.textContent = item.bar.value < 100 ? Math.floor(item.bar.value) + "%" : "storing ...";
		};
		xhr.onload = function () {
			done(xhr.status >= 200 && xhr.status < 300);
		};
		xhr.onerror = xhr.onabort = xhr.ontimeout = function () {
			done(false);
		};

		item.li.className = "";
		item.note.textContent = "0%";
		xhr.send(data);
	}

	function mark(item, ok) {
		item.ok = ok;
		if (ok) {
			item.bar.value = 100;
			item.note.textContent = humanSize(item.file.size);
			return;
		}

		item.li.className = "failed";
		item.note.textContent = "not stored";
		var retry = el("button", "", "retry");
		retry.type = "button";
		retry.onclick = function () {
			item.li.removeChild(retry);
			item.bar.value = 0;
			item.note.textContent = "waiting";
			queue.push(item);
			pump();
		};
		item.li.appendChild(retry);
	}

	// one file at a time: a phone on a weak link gains nothing from
	// sending two side by side, and the order on screen is the order sent
	function pump() {
		if (busy) {
			return;
		}
		var item = queue.shift();
		if (!item) {
			finish();
			return;
		}
		busy = true;
		summary.hidden = true;
		send(item, function (ok) {
			busy = false;
			mark(item, ok);
			pump();
		});
	}

	function finish() {
		var failed = 0, bytes = 0;
		items.forEach(function (item) {
			if (item.ok) {
				bytes += item.file.size;
			} else {
				failed++;
			}
		});
		var took = ((Date.now() - started) / 1000).toFixed(1) + "s";
		summary.textContent = (failed ? failed + " of " + items.length + " not stored" : "ok") +
			", received " + humanSize(bytes) + " in " + took;
		summary.hidden = false;
		more.hidden = false;
	}

	// css cannot tell a drag passing over the input, so the zone is
	// lit from here; the drop itself is the input's own
	var zone = input.parentNode;
	function over(on) {
		return function () {
			zone.classList.toggle("over", on);
		};
	}
	input.addEventListener("dragenter", over(true));
	input.addEventListener("dragleave", over(false));
	input.addEventListener("drop", over(false));

	form.addEventListener("submit", function (event) {
		// nothing picked: the form posts as it would without the script
		if (!input.files || input.files.length === 0) {
			return;
		}
		event.preventDefault();

		Array.prototype.forEach.call(input.files, function (file) {
			var item = add(file);
			items.push(item);
			queue.push(item);
		});
		form.hidden = true;
		list.hidden = false;
		started = Date.now();
		pump();
	});
})();

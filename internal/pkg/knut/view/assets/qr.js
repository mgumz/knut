// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

(function () {
	"use strict";

	var dialog = document.getElementById("qr-modal");
	if (!dialog || !dialog.showModal) {
		return;
	}

	var image = document.getElementById("qr-image"),
		caption = document.getElementById("qr-caption");

	function open(src, url) {
		image.src = src;
		image.alt = caption.textContent = url;
		dialog.showModal();
	}

	// the link is the entry itself, the code of it is that same uri with
	// "?qr" on the end - drawn by the server, asked for once it is shown
	document.addEventListener("click", function (event) {
		var link = event.target.closest && event.target.closest("a.qr-link");
		if (!link) {
			return;
		}
		event.preventDefault();

		var url = new URL(link.getAttribute("href"), location.href).href;
		open(url + "?qr", url);
	});

	// the code in the page header is already on the page: it is shown
	// larger, not asked for again. that is what makes it work on every
	// page, whether its handler answers "?qr" or not
	var code = document.querySelector("header img.qr");
	if (code) {
		code.classList.add("qr-open");
		code.addEventListener("click", function () {
			open(code.src, code.alt);
		});
	}

	dialog.addEventListener("click", function (event) {
		if (event.target === dialog) {
			dialog.close();
		}
	});

	dialog.addEventListener("close", function () {
		image.removeAttribute("src");
	});
})();

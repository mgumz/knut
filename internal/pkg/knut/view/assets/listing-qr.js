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

	// the link is the entry itself, the code of it is that same uri with
	// "?qr" on the end - drawn by the server, asked for once it is shown
	function open(href) {
		var url = new URL(href, location.href).href;

		image.src = url + "?qr";
		image.alt = caption.textContent = url;
		dialog.showModal();
	}

	document.addEventListener("click", function (event) {
		var link = event.target.closest && event.target.closest("a.qr-link");
		if (!link) {
			return;
		}
		event.preventDefault();
		open(link.getAttribute("href"));
	});

	// the code in the page header stands for the folder, as the "qr" in the
	// header of the listing does - but a bare folder has no listing header,
	// and a live one may fill up any moment, so it does not ask that link
	var code = document.querySelector("header img.qr");
	if (code) {
		code.classList.add("qr-open");
		code.addEventListener("click", function () {
			open("./");
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

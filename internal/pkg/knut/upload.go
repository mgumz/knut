// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

// UploadPartialDir is the folder inside an upload folder which a file is
// written into while it arrives. only once it is complete is it moved next
// to it, in one rename - so whoever looks at the upload folder sees whole
// files or none.
//
// the upload folder may be published by another mapping as well. a folder
// of this name is never published: not listed, not served, not packed into
// a zip or a tar.
const UploadPartialDir = ".knut-partial"

// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

// Tree is the tree knut throws through the window. plain ascii on purpose:
// it has to survive a terminal without a utf-8 locale as well as a browser
// without the fonts to draw anything fancier.
const Tree = `     *
    /o\
   /*o*\
  /o*o*o\
 /*o*o*o*\
    |_|`

// TreeGone is the same tree with the star down and the branches hanging:
// what is left of it where knut does not answer. same six lines and same
// width as Tree, so swapping one for the other moves nothing around it.
const TreeGone = `     .
    \ /
   \   /
  \     /
 \       /
    |_|`

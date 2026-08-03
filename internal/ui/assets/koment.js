(function () {
  "use strict";

  var switcher = document.querySelector("[data-switcher]");
  if (switcher) {
    switcher.addEventListener("change", function () {
      window.location.href = switcher.value;
    });
  }

  siftFiles();
  alignNotes();

  function siftFiles() {
    var box = document.querySelector("[data-sift]");
    var tree = document.querySelector("[data-tree]");
    if (!box || !tree) return;

    var files = Array.prototype.slice.call(tree.querySelectorAll(".file"));
    var dirs = Array.prototype.slice.call(tree.querySelectorAll(".dir"));
    var wasOpen = null;

    box.addEventListener("input", function () {
      var needle = box.value.trim().toLowerCase();

      if (needle && wasOpen === null) {
        wasOpen = dirs.map(function (dir) { return dir.open; });
      }

      files.forEach(function (file) {
        var match = !needle || file.dataset.name.toLowerCase().indexOf(needle) !== -1;
        file.hidden = !match;
      });

      dirs.forEach(function (dir, i) {
        var showing = dir.querySelector(".file:not([hidden])");
        dir.hidden = needle ? !showing : false;
        dir.open = needle ? true : (wasOpen ? wasOpen[i] : dir.open);
      });

      if (!needle) wasOpen = null;
    });
  }

  function alignNotes() {
    var reading = document.querySelector("[data-reading]");
    var gloss = document.querySelector("[data-gloss]");
    if (!reading || !gloss) return;

    var notes = Array.prototype.slice.call(gloss.querySelectorAll(".note[data-for]"));
    if (!notes.length) return;

    var narrow = window.matchMedia("(max-width: 900px)");
    var scheduled = null;

    function place() {
      scheduled = null;

      if (narrow.matches) {
        notes.forEach(function (note) { note.style.transform = ""; });
        gloss.classList.remove("aligned");
        return;
      }

      var top = gloss.getBoundingClientRect().top;
      var floor = 0;

      notes.forEach(function (note) {
        note.style.transform = "";
      });

      notes.forEach(function (note) {
        var row = document.getElementById("L" + note.dataset.for);
        if (!row) return;

        var wanted = row.getBoundingClientRect().top - top;
        var resting = note.offsetTop;
        var placed = Math.max(wanted, floor);

        note.style.transform = "translateY(" + (placed - resting) + "px)";
        floor = placed + note.offsetHeight + 12;
      });

      gloss.classList.add("aligned");
    }

    function schedule() {
      if (scheduled) return;
      scheduled = window.requestAnimationFrame(place);
    }

    place();
    window.addEventListener("resize", schedule);
    if (document.fonts && document.fonts.ready) document.fonts.ready.then(schedule);

    notes.forEach(function (note) {
      var row = document.getElementById("L" + note.dataset.for);
      if (!row) return;
      note.addEventListener("mouseenter", function () { row.classList.add("lit"); });
      note.addEventListener("mouseleave", function () { row.classList.remove("lit"); });
      row.addEventListener("mouseenter", function () { note.classList.add("lit"); });
      row.addEventListener("mouseleave", function () { note.classList.remove("lit"); });
    });
  }
})();

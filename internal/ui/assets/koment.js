(function () {
  "use strict";

  var switcher = document.querySelector("[data-switcher]");
  if (switcher) {
    switcher.addEventListener("change", function () {
      var destination = sameOriginDestination(switcher.value);
      if (destination) {
        window.location.assign(destination);
      }
    });
  }

  setupSearch();
  alignNotes();

  function sameOriginDestination(value) {
    if (typeof value !== "string" || value.length === 0) {
      return null;
    }
    var target;
    try {
      target = new URL(value, document.baseURI);
    } catch (error) {
      return null;
    }
    if (target.origin !== window.location.origin) {
      return null;
    }
    if (target.protocol !== "http:" && target.protocol !== "https:") {
      return null;
    }
    return target.href;
  }

  function setupSearch() {
    var dialog = document.querySelector("[data-search-dialog]");
    var trigger = document.querySelector("[data-search-open]");
    var close = document.querySelector("[data-search-close]");
    var input = document.querySelector("[data-search-input]");
    var results = document.querySelector("[data-search-results]");
    var tree = document.querySelector("[data-tree]");
    if (!dialog || !trigger || !close || !input || !results || !tree) return;

    var files = Array.prototype.slice.call(tree.querySelectorAll(".file"));
    var shortcut = document.querySelector("[data-search-shortcut]");
    var platform = navigator.userAgentData && navigator.userAgentData.platform || navigator.platform || navigator.userAgent;
    var apple = /Mac|iPhone|iPad|iPod/i.test(platform);
    var selected = 0;
    var rendered = [];
    var returnFocus = null;

    shortcut.textContent = apple ? "⌘K" : "Ctrl K";
    trigger.hidden = false;

    function editing(target) {
      if (!target) return false;
      var name = target.tagName;
      return target.isContentEditable || name === "INPUT" || name === "TEXTAREA" || name === "SELECT";
    }

    function matches() {
      var needle = input.value.trim().toLowerCase();
      var found = files.filter(function (file) {
        return !needle || file.dataset.search.indexOf(needle) !== -1;
      }).slice(0, 50);

      results.replaceChildren();
      rendered = found.map(function (file, index) {
        var result = document.createElement("a");
        var dot = document.createElement("span");
        var path = document.createElement("span");
        var meta = document.createElement("span");

        result.className = "search-result";
        result.href = file.href;
        result.setAttribute("role", "option");
        dot.className = "dot " + file.dataset.status;
        path.className = "search-result-path";
        path.textContent = file.dataset.path;
        meta.className = "search-result-meta";
        meta.textContent = file.dataset.count + (file.dataset.count === "1" ? " annotation" : " annotations");
        result.append(dot, path, meta);
        results.appendChild(result);
        if (index === selected) result.classList.add("selected");
        result.addEventListener("mousemove", function () { select(index); });
        return result;
      });

      if (!rendered.length) {
        var empty = document.createElement("p");
        empty.className = "search-empty";
        empty.textContent = "No matching annotations";
        results.appendChild(empty);
      }
      selected = Math.min(selected, Math.max(rendered.length - 1, 0));
      select(selected);
    }

    function select(index) {
      if (!rendered.length) return;
      selected = (index + rendered.length) % rendered.length;
      rendered.forEach(function (result, candidate) {
        var current = candidate === selected;
        result.classList.toggle("selected", current);
        result.setAttribute("aria-selected", current ? "true" : "false");
      });
      rendered[selected].scrollIntoView({ block: "nearest" });
    }

    function openSearch() {
      if (dialog.open) return;
      returnFocus = document.activeElement;
      selected = 0;
      input.value = "";
      matches();
      if (typeof dialog.showModal === "function") dialog.showModal();
      else dialog.setAttribute("open", "");
      input.focus();
    }

    function closeSearch() {
      if (!dialog.open) return;
      if (typeof dialog.close === "function") dialog.close();
      else dialog.removeAttribute("open");
      if (returnFocus && typeof returnFocus.focus === "function") returnFocus.focus();
    }

    trigger.addEventListener("click", openSearch);
    close.addEventListener("click", closeSearch);
    input.addEventListener("input", function () { selected = 0; matches(); });
    input.addEventListener("keydown", function (event) {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        select(selected + (event.key === "ArrowDown" ? 1 : -1));
      }
      if (event.key === "Enter" && rendered[selected]) {
        event.preventDefault();
        rendered[selected].click();
      }
    });
    dialog.addEventListener("click", function (event) {
      if (event.target === dialog) closeSearch();
    });
    dialog.addEventListener("close", function () {
      if (returnFocus && typeof returnFocus.focus === "function") returnFocus.focus();
    });
    document.addEventListener("keydown", function (event) {
      var modifier = apple ? event.metaKey : event.ctrlKey;
      if ((modifier && event.key.toLowerCase() === "k") || (event.key === "/" && !editing(event.target))) {
        event.preventDefault();
        openSearch();
      } else if (event.key === "Escape" && dialog.open) {
        event.preventDefault();
        closeSearch();
      }
    });

    matches();
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

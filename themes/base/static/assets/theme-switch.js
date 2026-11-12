(function () {
  var KEY = "ports-theme";
  var LABELS = { shrub: "Shrub's", xplshn: "Xplshn's" };
  var root = document.documentElement;
  var DEFAULT = LABELS[root.dataset.theme] ? root.dataset.theme : "shrub";

  function normalize(theme) {
    return LABELS[theme] ? theme : DEFAULT;
  }

  function apply(theme) {
    theme = normalize(theme);
    root.dataset.theme = theme;
    var links = document.querySelectorAll("link[data-theme-css]");
    for (var i = 0; i < links.length; i++) {
      links[i].media = links[i].getAttribute("data-theme-css") === theme ? "all" : "not all";
    }
    var hint = document.getElementById("theme-switcher");
    if (hint) {
      hint.hidden = false;
      hint.textContent = "Press [C] to change theme. Currently using " + LABELS[theme] + ".";
    }
  }

  function saved() {
    try { return localStorage.getItem(KEY); } catch (e) { return null; }
  }

  function blip() {
    root.classList.remove("theme-swap");
    void root.offsetWidth;
    root.classList.add("theme-swap");
    window.setTimeout(function () { root.classList.remove("theme-swap"); }, 200);
  }

  function store(theme) {
    try { localStorage.setItem(KEY, theme); } catch (e) {}
  }

  var current = normalize(saved() || DEFAULT);
  apply(current);

  document.addEventListener("DOMContentLoaded", function () {
    apply(current);
    document.addEventListener("keydown", function (e) {
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      if (!e.key || e.key.toLowerCase() !== "c") return;
      var t = e.target;
      var tag = t && t.tagName ? t.tagName.toLowerCase() : "";
      if (tag === "input" || tag === "textarea" || tag === "select" || (t && t.isContentEditable)) return;
      current = current === "shrub" ? "xplshn" : "shrub";
      store(current);
      blip();
      apply(current);
    });
  });
})();

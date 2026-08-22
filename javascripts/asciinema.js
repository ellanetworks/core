document.addEventListener("DOMContentLoaded", function () {
  document.querySelectorAll("[data-cast]").forEach(function (el) {
    AsciinemaPlayer.create(el.getAttribute("data-cast"), el, {
      autoPlay: true,
      loop: true,
      fit: "width",
    });
  });
});

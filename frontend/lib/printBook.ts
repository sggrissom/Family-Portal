export async function printBook() {
  const reader = document.querySelector(".book");
  if (!reader) return;
  const url = window.location.href;
  const images = Array.from(reader.querySelectorAll<HTMLImageElement>("img"));
  for (const img of images) img.loading = "eager";
  const loaded = Promise.all(
    images.map(img =>
      img.complete
        ? null
        : new Promise(resolve => {
            img.addEventListener("load", resolve, { once: true });
            img.addEventListener("error", resolve, { once: true });
          })
    )
  );
  await Promise.race([loaded, new Promise(resolve => setTimeout(resolve, 15000))]);
  if (
    reader.isConnected &&
    document.querySelector(".book") === reader &&
    window.location.href === url
  ) {
    window.print();
  }
}

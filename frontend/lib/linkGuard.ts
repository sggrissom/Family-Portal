// vlens routes every same-origin link click, even one a page's own handler
// already took over with preventDefault, which pushes the route a second
// time. Registered before vlens's listener, this keeps those clicks from it.
export function guardHandledLinkClicks(target: EventTarget = document) {
  target.addEventListener("click", event => {
    if (event.defaultPrevented) event.stopImmediatePropagation();
  });
}

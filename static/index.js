document.addEventListener('htmx:oobAfterSwap', function () {
  // htmx oob swaps do not currently support the scroll:bottom modifier
  const children = document.getElementById('chat-messages').children
  const lastChild = children[children.length - 1];
  lastChild.scrollIntoView();
});

document.addEventListener('htmx:wsError', function () {
  const div = document.createElement('div');
  div.classList.add('connection-error');
  div.appendChild(document.createTextNode('Connection error. You may need to reload this page.'));
  document.getElementById('chat-messages').appendChild(div);
  div.scrollIntoView();
});

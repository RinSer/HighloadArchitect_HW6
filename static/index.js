var loc = window.location;
var userIdRegEx = new RegExp('userId=([0-9]+)');

window.onload = async () => {
    if (!userIdRegEx?.test(loc.search)) {
        alert('URI path should contain query parameter userId!')
    } else {
        var userId = userIdRegEx.exec(loc.search)[1];

        var response = await fetch(
            '//' + loc.hostname + ':1234/feed/' + userId);
        var publications = await response.json();
        for (var publication of publications.reverse()) {
            addTableRow(publication);
        }

        var uri = 'ws:';
    
        if (loc.protocol === 'https:') {
            uri = 'wss:';
        }
        uri += '//' + loc.hostname + ':1234';
        uri += '/'+ userId + '/ws';
        
        ws = new WebSocket(uri)
        
        ws.onopen = () => console.log('WS Connected');
        
        ws.onmessage = async (evt) => addTableRow(
            JSON.parse(await evt.data.text())
        );
    }
};

function addTableRow(data) {
    var theader = document.getElementById('tableHeader');
    var tr = document.createElement('tr');
    var author = document.createElement('td');
    author.textContent = data['author'];
    var text = document.createElement('td');
    text.textContent = data['text'];
    var time = document.createElement('td');
    time.textContent = data['at'];
    tr.appendChild(author);
    tr.appendChild(text);
    tr.appendChild(time);
    theader.parentNode.insertBefore(tr, theader.nextSibling);
}
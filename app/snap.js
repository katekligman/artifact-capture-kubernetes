// Mostly From Wraith -- https://github.com/BBC-News/wraith/blob/master/LICENSE
var system = require('system');
var page = require('webpage').create();
var fs = require('fs');

if (system.args.length !== 5) {
    console.log('Usage: snap.js <URL> <view port width> <image path> <html path>');
    phantom.exit();
}

var url = system.args[1];
var view_port_width = system.args[2];
var image_name = system.args[3];
var html_name = system.args[4];
var current_requests = 0;
var last_request_timeout;
var final_timeout;
var status_code = -1;


page.viewportSize = { width: view_port_width, height: 1500};
page.settings = { loadImages: true, javascriptEnabled: true };

// If you want to use additional phantomjs commands, place them here
page.settings.userAgent = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) AppleWebKit/537.17 (KHTML, like Gecko) Chrome/28.0.1500.95 Safari/537.17';

page.onResourceRequested = function(req) {
  current_requests += 1;
};

page.onResourceReceived = function(res) {
  if (res.stage === 'end') {
    global.status_code = res.status;
    current_requests -= 1;
    debounced_render();
  }
};

page.open(url, function(status) {
  if (status !== 'success') {
    console.log('Error with page ' + url);
    phantom.exit();
  }
});

function debounced_render() {
  clearTimeout(last_request_timeout);
  clearTimeout(final_timeout);

  // If there's no more ongoing resource requests, wait for 5s before
  // rendering, just in case the page kicks off another request
  if (current_requests < 1) {
      clearTimeout(final_timeout);
      last_request_timeout = setTimeout(function() {
          page.render(image_name);
          fs.write(html_name, page.content);
          system.stdout.write(global.status_code);
          phantom.exit();
      }, 5000);
  }

  // Sometimes, straggling requests never make it back, in which
  // case, timeout after 20 seconds and render the page anyway
  final_timeout = setTimeout(function() {
    page.render(image_name);
    fs.write(html_name, page.content);
    system.stdout.write(global.status_code);
    phantom.exit();
  }, 20000);
}

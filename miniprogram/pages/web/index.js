const settings=require('../../config');
Page({data:{url:'',error:''},onLoad(options){let url;try{url=decodeURIComponent(options.url||'');}catch{this.setData({error:'链接格式不正确'});return;}const match=url.match(/^https:\/\/([^/?#]+)/);if(!match||!settings.H5_WEBVIEW_HOSTS.includes(match[1]))this.setData({error:'该链接尚未配置为可在小程序内使用的业务域名。'});else this.setData({url});}});

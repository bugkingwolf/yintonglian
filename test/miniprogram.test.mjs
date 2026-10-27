import {test} from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync,readdirSync} from 'node:fs';

test('all conditional WXML branches bind state instead of truthy literal strings',()=>{
 for(const page of readdirSync('miniprogram/pages')){
  const template=readFileSync(`miniprogram/pages/${page}/index.wxml`,'utf8');
  for(const [,expression] of template.matchAll(/wx:(?:if|elif)="([^"]*)"/g)){
   assert.match(expression,/^\{\{.+\}\}$/s,`${page}: literal condition ${expression}`);
   assert.doesNotMatch(expression,/&amp;/,`${page}: HTML entity inside expression`);
  }
 }
});

function home(api){
 let page;
 vm.runInNewContext(readFileSync('miniprogram/pages/home/index.js','utf8'),{
  require:()=>api,Page:p=>{page=p;},wx:{stopPullDownRefresh(){}}
 });
 page.setData=values=>Object.assign(page.data,values);
 return page;
}

test('home exits loading on success and exposes a retryable error on failure',async()=>{
 const page=home({content:async()=>({config:{company:{homeFeaturedLimit:4}},items:[{id:'sample',featured:true,hero:true}]})});
 await page.load();
 assert.equal(page.data.loading,false);
 assert.equal(page.data.hero.id,'sample');
 const failed=home({content:async()=>{throw Error('网络不可用');}});
 await failed.load();
 assert.equal(failed.data.loading,false);
 assert.equal(failed.data.error,'网络不可用');
});

test('API requests have finite timeout and handle empty error responses',async()=>{
 const apiModule={exports:{}};
 let request;
 vm.runInNewContext(readFileSync('miniprogram/utils/api.js','utf8'),{
  require:()=>({API_BASE:'https://example.com'}),module:apiModule,wx:{request:opts=>{request=opts;}}
 });
 const failed=apiModule.exports.request('/api/public/config');
 assert.equal(request.timeout,12000);
 request.success({statusCode:502,data:null});
 await assert.rejects(failed,/加载失败/);
 const network=apiModule.exports.request('/api/public/cases');
 request.fail();
 await assert.rejects(network,/连接暂不可用/);
 const domain=apiModule.exports.request('/api/public/config');
 request.fail({errMsg:'request:fail url not in domain list'});
 await assert.rejects(domain,/合法域名/);
});

test('platform opens configured mobile URL with hash intact and keeps copy fallback',()=>{
 let page,target,copied;
 vm.runInNewContext(readFileSync('miniprogram/pages/platforms/index.js','utf8'),{
  require:()=>({}),Page:p=>{page=p;},wx:{navigateTo:p=>{target=p.url;},setClipboardData:p=>{copied=p.data;}}
 });
 page.data.config={platforms:[{id:'comic',url:'https://old.example',mobileUrl:'https://aiyfj.com/#/login'}]};
 const event={currentTarget:{dataset:{id:'comic'}}};
 page.open(event);
 assert.equal(decodeURIComponent(target.split('?url=')[1]),'https://aiyfj.com/#/login');
 page.copy(event);
 assert.equal(copied,'https://aiyfj.com/#/login');
});

test('webview accepts permitted HTTPS hosts and rejects other schemes and hosts',()=>{
 let page;
 vm.runInNewContext(readFileSync('miniprogram/pages/web/index.js','utf8'),{
  require:()=>({H5_WEBVIEW_HOSTS:['aiyfj.com','user.jiougeo.com']}),Page:p=>{page=p;}
 });
 page.setData=values=>Object.assign(page.data,values);
 page.onLoad({url:encodeURIComponent('https://aiyfj.com/#/login')});
 assert.equal(page.data.url,'https://aiyfj.com/#/login');
 for(const url of ['http://aiyfj.com','https://aiyfj.com.evil.example','https://aiyfj.com@evil.example']){
  page.data={url:'',error:''};page.onLoad({url:encodeURIComponent(url)});
  assert.equal(page.data.url,'');assert.ok(page.data.error);
 }
});

test('member card shares its stable path and only shows available case records',async()=>{
 let page,saved,phone,copied,preview,shared;
 const member={id:'person-one',name:'联系人',role:'商务',phone:'010-12345678',wechat:'wechat_test',wechatQR:'https://example.com/qr.png',caseIds:['published','missing']};
 vm.runInNewContext(readFileSync('miniprogram/pages/member/index.js','utf8'),{
 require:()=>({content:async()=>({config:{company:{name:'银通联',teamMembers:[member]}},items:[{id:'published'}]})}),Page:p=>{page=p;},wx:{stopPullDownRefresh(){},setNavigationBarTitle(){},addPhoneContact:p=>{saved=p;},makePhoneCall:p=>{phone=p;},setClipboardData:p=>{copied=p;},previewImage:p=>{preview=p;}}
 });page.setData=v=>Object.assign(page.data,v);page.id='person-one';await page.load();
 assert.equal(page.data.items.length,1);assert.equal(page.data.loading,false);
 page.saveContact();assert.equal(saved.firstName,'联系人');assert.equal(saved.weChatNumber,'wechat_test');
 page.phone();assert.equal(phone.phoneNumber,'01012345678');page.copyWechat();assert.equal(copied.data,'wechat_test');page.previewQR();assert.equal(preview.current,member.wechatQR);
 assert.equal(page.onShareAppMessage().path,'/pages/member/index?id=person-one');
 page.id='hidden';await page.load();assert.equal(page.data.member,null);assert.match(page.data.error,/未公开/);assert.equal(page.onShareAppMessage().path,'/pages/about/index');
});

test('share cover uses a 5:4 canvas with identity text and a contained portrait',async()=>{
 let module={exports:{}},exported,drawn=[],text=[];
 const ctx={fillRect(){},fillText(...args){text.push(args);},measureText:s=>({width:s.length*12}),drawImage(...args){drawn.push(args);}};
 const canvas={getContext:()=>ctx,createImage:()=>{const image={width:300,height:600};Object.defineProperty(image,'src',{set(){image.onload();}});return image;}};
 const wx={createSelectorQuery:()=>({in(){return this;},select(){return this;},fields(){return this;},exec(fn){fn([{node:canvas}]);}}),getImageInfo:opts=>opts.success({path:'local-photo'}),canvasToTempFilePath:opts=>{exported=opts;opts.success({tempFilePath:'wxfile://business-card.png'});}};
 vm.runInNewContext(readFileSync('miniprogram/utils/member-share.js','utf8'),{module,wx,setTimeout,clearTimeout});
 const output=await module.exports.createMemberCover({}, {name:'张先生',role:'商务总监',photo:'https://example.com/photo.jpg'}, {name:'银通联',english:'MEDIA BANK'});
 assert.equal(output,'wxfile://business-card.png');assert.equal(exported.destWidth/exported.destHeight,1.25);
 assert.ok(text.some(args=>args[0]==='张先生'));assert.ok(text.some(args=>args[0]==='商务总监'));assert.ok(text.some(args=>args[0]==='银通联'));
 assert.equal(drawn.length,1);assert.ok(drawn[0][3]<=174);assert.ok(drawn[0][4]<=220);
 wx.getImageInfo=opts=>opts.fail();drawn=[];text=[];
 assert.equal(await module.exports.createMemberCover({}, {name:'张先生',role:'商务',photo:'bad'}, {name:'银通联'}),'wxfile://business-card.png');assert.equal(drawn.length,0);assert.ok(text.some(args=>args[0]==='张先生'));
});

test('sharing waits for the generated card instead of returning the raw portrait',async()=>{
 let page;
 vm.runInNewContext(readFileSync('miniprogram/pages/member/index.js','utf8'),{require:()=>({}),Page:p=>{page=p;},Promise});
 page.data={member:{id:'person',name:'张先生',role:'商务',photo:'https://example.com/head.jpg'},company:{name:'银通联'}};
 page.shareCover=Promise.resolve('wxfile://designed-card.png');
 const share=page.onShareAppMessage();assert.equal(share.imageUrl,undefined);
 const ready=await share.promise;assert.equal(ready.imageUrl,'wxfile://designed-card.png');assert.equal(ready.path,'/pages/member/index?id=person');
 page.shareImage='wxfile://cached-card.png';assert.equal(page.onShareAppMessage().imageUrl,'wxfile://cached-card.png');
});

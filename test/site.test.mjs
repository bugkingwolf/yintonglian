import {test} from 'node:test';import assert from 'node:assert/strict';import vm from 'node:vm';import {readFileSync,existsSync} from 'node:fs';const seed=JSON.parse(readFileSync('internal/app/seed.json','utf8'));
function environment(){
 const nodes=new Map();const node=key=>{if(!nodes.has(key))nodes.set(key,{innerHTML:'',textContent:'',value:'',dataset:{},classList:{add(){},remove(){}},addEventListener(){},querySelector(){return node(key+'/child');},showModal(){this.open=true;},close(){this.open=false;}});return nodes.get(key);};
 const location={pathname:'/',search:'',origin:'http://localhost',href:'http://localhost/'};
 const setRoute=url=>{const u=new URL(url,location.origin);Object.assign(location,{pathname:u.pathname,search:u.search,href:u.href});};
 const document={querySelector:node,querySelectorAll:()=>[],addEventListener(){},title:''};
 const context=vm.createContext({document,location,history:{pushState(_,__,url){setRoute(url);},replaceState(_,__,url){setRoute(url);}},window:{scrollTo(){},addEventListener(){},open(){}},fetch:async url=>{const u=new URL(url,'http://localhost'),q=u.searchParams;let value;if(u.pathname==='/api/public/config')value={categories:seed.categories,clients:seed.clients,company:seed.company,platforms:seed.platforms};else if(u.pathname==='/api/public/home')value={hero:seed.cases.filter(x=>x.hero).slice(0,1),featured:seed.cases.filter(x=>x.featured).slice(0,4),categoryCases:seed.categories.map(x=>seed.cases.find(c=>c.categoryId===x.id)).filter(Boolean),clientIds:seed.clients.map(c=>c.id)};else if(u.pathname==='/api/public/case-list'){let items=seed.cases.filter(c=>(!q.get('category')||c.categoryId===q.get('category'))&&(!q.get('client')||c.clientId===q.get('client'))&&(!q.get('type')||c.type===q.get('type')));if(q.get('relatedTo'))items=items.filter(x=>x.id!==q.get('relatedTo')).slice(0,2);value={items,total:items.length,page:1,pageSize:12};}else if(u.pathname.startsWith('/api/public/cases/'))value=seed.cases.find(x=>x.id===decodeURIComponent(u.pathname.split('/').pop()));return {ok:!!value,status:value?200:404,json:async()=>value||{error:'案例暂不可查看'}}},navigator:{clipboard:{writeText:async()=>{}}},URLSearchParams,URL,setTimeout,clearTimeout,matchMedia:()=>({matches:false}),FormData,console});
 vm.runInContext(readFileSync('public/site.js','utf8'),context);return {context,nodes,node,ready:async()=>{await new Promise(r=>setTimeout(r,1));},go:path=>vm.runInContext(`navigate(${JSON.stringify(path)})`,context),html:()=>node('#app').innerHTML};
}
test('website renders real category and case paths without phantom media',async()=>{const e=environment();await e.ready();assert.match(e.html(),/探索业务与案例/);assert.match(e.html(),/\/cases\?category=marketing/);assert.match(e.html(),/\/cases\/life-forward/);assert.match(e.html(),/业务与案例/);for(const c of seed.cases)assert.ok(existsSync('public'+c.cover));await e.go('/cases');assert.match(e.html(),/查看营销案例/);assert.match(e.html(),/按客户看案例/);await e.go('/cases?category=technology');assert.match(e.html(),/儿童节绘画互动/);assert.doesNotMatch(e.html(),/class="work-card video"/);await e.go('/cases?client=icbc');assert.match(e.html(),/工商银行案例/);assert.match(e.html(),/3 个案例/);await e.go('/cases/life-forward');assert.match(e.html(),/原视频上传后可在线播放/);assert.doesNotMatch(e.html(),/<video controls/);assert.match(e.html(),/分享案例/);await e.go('/platforms');assert.match(e.html(),/平台入口待配置/);assert.doesNotMatch(e.html(),/data-platform="geo"/);await e.go('/about');assert.match(e.html(),/公司愿景/);assert.match(e.html(),/4008-125521/);await e.go('/cases/nonexistent');assert.match(e.html(),/案例暂不可查看/);});
test('public text is escaped rather than rendered as HTML',async()=>{const e=environment();await e.ready();vm.runInContext("config.company.slogan='<img src=x onerror=alert(1)>'; render();",e.context);assert.match(e.html(),/&lt;img/);assert.doesNotMatch(e.html(),/<img src=x onerror/);});

test('business card contains only personal information and contact actions',async()=>{
 const e=environment();await e.ready();
 vm.runInContext(`config.company.teamMembers=[{id:'card-test',name:'示例人员',role:'商务',biography:'<img src=x onerror=alert(1)>',phone:'010-12345678',wechat:'test_wechat',wechatQR:'/media/example.png',caseIds:['life-forward','missing'],group:'execution'}];render();`,e.context);
 await e.go('/about');assert.match(e.html(),/\/people\/card-test/);
 await e.go('/people/card-test');assert.match(e.html(),/保存联系人/);assert.match(e.html(),/tel:01012345678/);assert.match(e.html(),/复制微信号/);assert.doesNotMatch(e.html(),/代表案例|SELECTED PROJECTS|class="work-card/);assert.match(e.html(),/&lt;img/);assert.doesNotMatch(e.html(),/<img src=x onerror/);assert.doesNotMatch(e.html(),/\/cases\/missing/);
 await e.go('/people/hidden');assert.match(e.html(),/名片暂不可查看/);assert.doesNotMatch(e.html(),/test_wechat/);
});

test('company navigation links use configured coordinates and disappear when cleared',async()=>{
 const e=environment();await e.ready();await e.go('/about');assert.match(e.html(),/导航到公司/);assert.match(e.html(),/position=116.403885,39.99397/);
 vm.runInContext('config.company.latitude=null;config.company.longitude=null;render()',e.context);assert.doesNotMatch(e.html(),/导航到公司/);
});

test('route requests discard stale filters and recover from network errors',async()=>{
 const e=environment();await e.ready();const pending=[];
 e.context.fetch=url=>new Promise(resolve=>pending.push({url,resolve}));
 const old=e.go('/cases?type=video'),latest=e.go('/cases?type=graphic');
 assert.equal(pending.length,2);assert.ok(pending.every(x=>x.url.startsWith('/api/public/case-list?')));
 pending[1].resolve({ok:true,json:async()=>({items:[seed.cases.find(c=>c.type==='graphic')],total:1,page:1,pageSize:12})});await latest;
 pending[0].resolve({ok:true,json:async()=>({items:[seed.cases[0]],total:1,page:1,pageSize:12})});await old;
 assert.match(e.html(),/class="work-card graphic"/);assert.doesNotMatch(e.html(),/class="work-card video"/);
 e.context.fetch=async()=>({ok:false,status:503,json:async()=>({error:'网络测试失败'})});await e.go('/cases?view=all');assert.match(e.html(),/网络测试失败/);assert.match(e.html(),/data-retry-route/);
 e.context.fetch=async()=>({ok:true,json:async()=>({items:[seed.cases[0]],total:30,page:2,pageSize:12})});await e.go('/cases?view=all&page=2');assert.match(e.html(),/30 个案例/);assert.match(e.html(),/第 2 \/ 3 页/);assert.match(e.html(),/page=3/);
 let requests=0;e.context.fetch=async()=>{requests++;throw Error('should not request cases');};await e.go('/about');await e.go('/platforms');assert.equal(requests,0);
});

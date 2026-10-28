const settings=require('../config');
function request(path){return new Promise((resolve,reject)=>wx.request({url:settings.API_BASE+path,method:'GET',timeout:12000,success(res){if(res.statusCode>=200&&res.statusCode<300)resolve(res.data);else reject(new Error((res.data&&res.data.error)||'加载失败'));},fail(err={}){const detail=err.errMsg||'';reject(new Error(/domain|合法域名/i.test(detail)?'服务器域名尚未加入微信合法域名，请联系管理员配置后重试。':/timeout/i.test(detail)?'连接超时，请稍后重试。':'连接暂不可用，请稍后重试。'));}}));}
function media(value){return value&&value.startsWith('/')?settings.API_BASE+value:value;}
let cachedConfig=null,configExpires=0,configPromise=null,configVersion=0;
async function loadConfig(force=false){
 if(!force&&cachedConfig&&Date.now()<configExpires)return cachedConfig;
 if(!force&&configPromise)return configPromise;
 const version=++configVersion;
 const pending=request('/api/public/config').then(value=>{value.company.qualificationImages=(value.company.qualificationImages||[]).map(media);value.company.teamMembers=(value.company.teamMembers||[]).map(m=>({...m,photo:media(m.photo),wechatQR:media(m.wechatQR)}));if(version===configVersion){cachedConfig=value;configExpires=Date.now()+120000;}return value;});
 configPromise=pending;try{return await pending;}finally{if(configPromise===pending)configPromise=null;}
}
function decorate(c,config){return {...c,cover:media(c.cover),gallery:(c.gallery||[]).map(media),video:media(c.video),clientName:config.clients.find(x=>x.id===c.clientId)?.name||'银通联项目',categoryName:config.categories.find(x=>x.id===c.categoryId)?.name||'业务案例',typeName:{video:'视频',graphic:'图文',h5:'H5互动',mixed:'整合案例'}[c.type]};}
function query(values){return Object.entries(values).filter(([,v])=>v!==''&&v!=null).map(([k,v])=>encodeURIComponent(k)+'='+encodeURIComponent(v)).join('&');}
async function caseList(values,config){config=config||await loadConfig();const page=await request('/api/public/case-list?'+query(values));return {...page,items:page.items.map(c=>decorate(c,config))};}
async function home(force=false){const [config,value]=await Promise.all([loadConfig(force),request('/api/public/home')]);return {config,hero:value.hero.map(c=>decorate(c,config)),featured:value.featured.map(c=>decorate(c,config))};}
async function detail(id,force=false){const [config,item,related]=await Promise.all([loadConfig(force),request('/api/public/cases/'+encodeURIComponent(id)),request('/api/public/case-list?'+query({relatedTo:id,pageSize:2}))]);return {config,item:decorate(item,config),related:related.items.map(c=>decorate(c,config))};}
function contact(company){if(company.phone)wx.showModal({title:'联系银通联',content:(company.phone||'')+'\n'+(company.address||''),confirmText:'拨打电话',success(r){if(r.confirm)wx.makePhoneCall({phoneNumber:company.phone.replace(/[^\d+]/g,'')});}});else wx.showModal({title:'联系银通联',content:company.address||'联系方式待补充',showCancel:false});}
function openCases(filter){getApp().globalData.pendingFilter=filter||{};wx.switchTab({url:'/pages/cases/index'});}
function hasLocation(company){return typeof company?.latitude==='number'&&typeof company?.longitude==='number'&&Number.isFinite(company.latitude)&&Number.isFinite(company.longitude)&&Math.abs(company.latitude)<=90&&Math.abs(company.longitude)<=180;}
function openLocation(company){if(!hasLocation(company))return;wx.openLocation({latitude:company.latitude,longitude:company.longitude,name:company.name,address:company.address,scale:18,fail(){wx.showModal({title:'地图暂未打开',content:company.address,confirmText:'复制地址',success(r){if(r.confirm)wx.setClipboardData({data:company.address});}});}});}
module.exports={request,media,config:loadConfig,caseList,home,detail,contact,openCases,hasLocation,openLocation};

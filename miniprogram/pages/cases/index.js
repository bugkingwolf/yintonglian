const api=require('../../utils/api');
Page({
 data:{loading:true,error:'',config:null,items:[],total:0,page:0,hasMore:false,listLoading:false,listError:'',categoryId:'',clientId:'',type:'',query:'',overview:true,clients:[],categories:[],clientIndex:0,categoryIndex:0,types:[{id:'',name:'全部'},{id:'video',name:'视频'},{id:'graphic',name:'图文'},{id:'h5',name:'H5互动'},{id:'mixed',name:'整合案例'}]},
 onShow(){const pending=getApp().globalData.pendingFilter;if(!this.data.config||pending)this.load(false,pending);},
 onPullDownRefresh(){this.load(true);},
 onReachBottom(){if(!this.data.overview&&this.data.hasMore&&!this.data.listLoading)this.loadPage(true);},
 onUnload(){this.sequence=(this.sequence||0)+1;},
 async load(force=false,pending){
  this.sequence=(this.sequence||0)+1;const sequence=this.sequence;this.setData({loading:true,error:'',listLoading:false});
  if(pending)getApp().globalData.pendingFilter=null;
  try{const config=await api.config(force);if(sequence!==this.sequence)return;
   const filter=pending?{categoryId:pending.categoryId||'',clientId:pending.clientId||'',type:'',query:'',overview:!pending.all&&!pending.categoryId&&!pending.clientId}:{};
   this.setData({config,categories:[{id:'',name:'全部业务'},...config.categories],clients:[{id:'',name:'全部客户'},...config.clients],loading:false,...filter});this.labels();if(!this.data.overview)await this.loadPage(false);
  }catch(e){if(sequence===this.sequence)this.setData({error:e.message,loading:false});}finally{wx.stopPullDownRefresh();}
 },
 labels(){const d=this.data;this.setData({categoryIndex:Math.max(0,d.categories.findIndex(x=>x.id===d.categoryId)),clientIndex:Math.max(0,d.clients.findIndex(x=>x.id===d.clientId)),selectedTitle:d.clientId?(d.clients.find(x=>x.id===d.clientId)?.name||'客户')+'案例':d.categoryId?d.categories.find(x=>x.id===d.categoryId)?.name:'全部案例'});},
 async loadPage(append=false){
  if(append&&this.data.listLoading)return;
  const sequence=append?this.sequence:(this.sequence=(this.sequence||0)+1),d=this.data;
  this.setData({listLoading:true,listError:'',...(append?{}:{items:[],total:0,page:0,hasMore:false})});
  try{const result=await api.caseList({page:append?d.page+1:1,pageSize:12,category:d.categoryId,client:d.clientId,type:d.type,q:d.query},d.config);if(sequence!==this.sequence)return;
   const all=append?[...this.data.items,...result.items]:result.items,seen=new Set();const items=all.filter(x=>!seen.has(x.id)&&seen.add(x.id));
   this.setData({items,total:result.total,page:result.page,hasMore:result.page*result.pageSize<result.total});
  }catch(e){if(sequence===this.sequence)this.setData({listError:e.message});}finally{if(sequence===this.sequence)this.setData({listLoading:false});}
 },
 apply(){this.labels();if(!this.data.overview)this.loadPage(false);},
 retryList(){this.loadPage(this.data.items.length>0);},
 category(e){this.setData({categoryId:e.currentTarget.dataset.id,overview:false});this.apply();},
 client(e){this.setData({clientId:e.currentTarget.dataset.id,overview:false});this.apply();},
 all(){this.setData({overview:false,categoryId:'',clientId:'',type:'',query:''});this.apply();},
 reset(){this.sequence=(this.sequence||0)+1;this.setData({overview:true,categoryId:'',clientId:'',type:'',query:'',items:[],total:0,page:0,listLoading:false,listError:''});this.labels();},
 changeCategory(e){this.setData({categoryId:this.data.categories[Number(e.detail.value)].id});this.apply();},
 changeClient(e){this.setData({clientId:this.data.clients[Number(e.detail.value)].id});this.apply();},
 changeType(e){this.setData({type:e.currentTarget.dataset.id});this.apply();},
 search(e){this.setData({query:e.detail.value.trim()});this.apply();},
 openCase(e){wx.navigateTo({url:'/pages/detail/index?id='+encodeURIComponent(e.currentTarget.dataset.id)});},
 onShareAppMessage(){return {title:'银通联 · 业务与案例',path:'/pages/cases/index'};}
});

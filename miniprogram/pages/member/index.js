const api=require('../../utils/api');
Page({
 data:{loading:true,error:'',member:null,company:null,items:[]},
 onLoad(options){this.id=options.id||'';this.load();},
 onPullDownRefresh(){this.load();},
 async load(){this.setData({loading:true,error:''});try{const {config,items}=await api.content();const member=config.company.teamMembers.find(m=>m.id===this.id);if(!member)throw Error('该名片暂未公开或已停止展示。');this.setData({member,company:config.company,items:(member.caseIds||[]).map(id=>items.find(c=>c.id===id)).filter(Boolean),loading:false});wx.setNavigationBarTitle({title:member.name+'的名片'});}catch(e){this.setData({member:null,items:[],error:e.message,loading:false});}finally{wx.stopPullDownRefresh();}},
 phone(){const value=this.data.member?.phone;if(value)wx.makePhoneCall({phoneNumber:value.replace(/[^\d+]/g,''),fail(){wx.showToast({title:'未完成拨号',icon:'none'});}});},
 copyWechat(){const value=this.data.member?.wechat;if(value)wx.setClipboardData({data:value});},
 previewQR(){const url=this.data.member?.wechatQR;if(url)wx.previewImage({current:url,urls:[url]});},
 saveContact(){const m=this.data.member;if(!m)return;wx.addPhoneContact({firstName:m.name,mobilePhoneNumber:m.phone||'',organization:this.data.company.legalName||this.data.company.name,title:m.role,weChatNumber:m.wechat||'',fail(){wx.showModal({title:'未保存联系人',content:'你可以使用拨打电话、复制微信号或查看二维码联系对方。',showCancel:false});}});},
 openCase(e){wx.navigateTo({url:'/pages/detail/index?id='+encodeURIComponent(e.currentTarget.dataset.id)});},
 onShareAppMessage(){const m=this.data.member;return m?{title:m.name+' · '+m.role+'｜'+this.data.company.name,path:'/pages/member/index?id='+encodeURIComponent(m.id),...(m.photo?{imageUrl:m.photo}:{})}:{title:'银通联',path:'/pages/about/index'};}
});

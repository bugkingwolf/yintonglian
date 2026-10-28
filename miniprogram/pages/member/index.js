const api=require('../../utils/api');
const {createMemberCover}=require('../../utils/member-share');
Page({
 data:{loading:true,error:'',member:null,company:null},
 onLoad(options){this.id=options.id||'';this.load();},
 onPullDownRefresh(){this.load(true);},
 async load(force=false){this.shareImage='';this.shareCover=null;this.setData({loading:true,error:''});try{const config=await api.config(force);const member=config.company.teamMembers.find(m=>m.id===this.id);if(!member)throw Error('该名片暂未公开或已停止展示。');this.setData({member,company:config.company,loading:false},()=>{this.shareCover=createMemberCover(this,member,config.company).then(path=>{if(this.data.member===member)this.shareImage=path;return path;});});wx.setNavigationBarTitle({title:member.name+'的名片'});}catch(e){this.setData({member:null,error:e.message,loading:false});}finally{wx.stopPullDownRefresh();}},
 phone(){const value=this.data.member?.phone;if(value)wx.makePhoneCall({phoneNumber:value.replace(/[^\d+]/g,''),fail(){wx.showToast({title:'未完成拨号',icon:'none'});}});},
 copyWechat(){const value=this.data.member?.wechat;if(value)wx.setClipboardData({data:value});},
 previewQR(){const url=this.data.member?.wechatQR;if(url)wx.previewImage({current:url,urls:[url]});},
 saveContact(){const m=this.data.member;if(!m)return;wx.addPhoneContact({firstName:m.name,mobilePhoneNumber:m.phone||'',organization:this.data.company.legalName||this.data.company.name,title:m.role,weChatNumber:m.wechat||'',fail(){wx.showModal({title:'未保存联系人',content:'你可以使用拨打电话、复制微信号或查看二维码联系对方。',showCancel:false});}});},
 onShareAppMessage(){const m=this.data.member;if(!m)return {title:'银通联',path:'/pages/about/index'};const share={title:m.name+' · '+m.role+'｜'+this.data.company.name,path:'/pages/member/index?id='+encodeURIComponent(m.id)};if(this.shareImage)return {...share,imageUrl:this.shareImage};return {...share,promise:(this.shareCover||Promise.resolve('')).then(imageUrl=>imageUrl?{...share,imageUrl}:share)};}
});

// A dedicated 5:4 share cover; never use the original portrait as a cover.
function createMemberCover(page,member,company){
 return new Promise(resolve=>{
  if(typeof wx.createSelectorQuery!=='function'){resolve('');return;}
  let done=false;const finish=path=>{if(!done){done=true;clearTimeout(timer);resolve(path||'');}};
  const timer=setTimeout(()=>finish(''),2200);
  try{wx.createSelectorQuery().in(page).select('#member-share-canvas').fields({node:true,size:true}).exec(result=>{
   try{
    const canvas=result[0]?.node;if(!canvas){finish('');return;}
    canvas.width=600;canvas.height=480;const ctx=canvas.getContext('2d');
    const text=(value,x,y,max,size,color)=>{ctx.font=size+'px sans-serif';ctx.fillStyle=color;let s=String(value||'').replace(/[\r\n]/g,' ');while(s.length>1&&ctx.measureText(s).width>max)s=s.slice(0,-2)+'…';ctx.fillText(s,x,y);};
    const draw=image=>{
     if(done)return;
     ctx.fillStyle='#faf8f3';ctx.fillRect(0,0,600,480);
     ctx.fillStyle='#a27b38';ctx.fillRect(34,36,4,39);
     text(company.english||'MEDIA BANK',52,52,490,18,'#9a712d');
     text(company.name,52,79,490,20,'#292e28');
     ctx.fillStyle='#e9e1d2';ctx.fillRect(34,106,532,1);
     // Keep the entire portrait in its own small panel instead of cropping a face.
     ctx.fillStyle='#eee8dc';ctx.fillRect(34,137,174,220);
     if(image){const ratio=Math.min(174/image.width,220/image.height),w=image.width*ratio,h=image.height*ratio;ctx.drawImage(image,34+(174-w)/2,137+(220-h)/2,w,h);}
     else text((member.name||'').slice(0,1),91,266,80,50,'#9a712d');
     text(member.name,239,212,325,42,'#292e28');
     text(member.role,241,255,322,24,'#9a712d');
     text('个人名片',241,314,320,18,'#777970');
     ctx.fillStyle='#e9e1d2';ctx.fillRect(34,389,532,1);
     text('认识团队 · 了解作品 · 洽谈合作',34,432,475,18,'#777970');
     text('→',535,435,35,27,'#9a712d');
     wx.canvasToTempFilePath({canvas,width:600,height:480,destWidth:600,destHeight:480,fileType:'png',success:r=>finish(r.tempFilePath),fail:()=>finish('')},page);
    };
    if(!member.photo){draw();return;}
    // If the photo cannot be loaded, export a complete text card instead.
    let painted=false;let photoTimer;
    const paint=image=>{if(painted)return;painted=true;clearTimeout(photoTimer);draw(image);};
    photoTimer=setTimeout(()=>paint(),1200);
    wx.getImageInfo({src:member.photo,success(info){try{const img=canvas.createImage();img.onload=()=>paint(img);img.onerror=()=>paint();img.src=info.path;}catch{paint();}},fail:()=>paint()});
   }catch{finish('');}
  });}catch{finish('');}
 });
}
module.exports={createMemberCover};

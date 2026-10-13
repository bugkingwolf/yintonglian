import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
const {company,categories,clients,cases,platforms}=JSON.parse(readFileSync('internal/app/seed.json','utf8'));
const config={company,categories,clients,platforms},items=JSON.parse(JSON.stringify(cases));
for(const c of items){c.cover='../public'+c.cover;c.gallery=c.gallery.map(x=>'../public'+x);}
let script=readFileSync('public/site.js','utf8').replaceAll('location.pathname','previewLocation.pathname').replaceAll('location.search','previewLocation.search').replaceAll("history.replaceState({},'',url)","setRoute(url)").replaceAll("history.pushState({},'',url)","setRoute(url)").replace('href="/admin"','href="../README.md"');
const data=JSON.stringify({config,items}).replaceAll('<','\\u003c');
const setup=`const previewData=${data};const previewLocation={pathname:'/',search:''};function setRoute(url){const u=new URL(url,'https://preview.invalid');previewLocation.pathname=u.pathname;previewLocation.search=u.search;}const fetch=async(path)=>new Response(JSON.stringify(path==='/api/public/config'?previewData.config:{items:previewData.items}),{status:200,headers:{'Content-Type':'application/json'}});`;
const base=readFileSync('public/index.html','utf8').replace('<link rel="stylesheet" href="/site.css">','<style>'+readFileSync('public/site.css','utf8')+'</style>').replace('<script type="module" src="/site.js"></script>','').replace('</body>','<script>(()=>{'+setup+'\n'+script+'})()</script></body>').replace('<body>','<body><div style="background:#292c27;color:#ddd;text-align:center;font-size:11px;padding:5px 10px">界面只读预览 · 后台与正式接口请按运行说明启动</div>');
mkdirSync('preview',{recursive:true});writeFileSync('preview/index.html',base);console.log('Read-only preview prepared: preview/index.html');

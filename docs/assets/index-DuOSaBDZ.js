(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))o(s);new MutationObserver(s=>{for(const r of s)if(r.type==="childList")for(const i of r.addedNodes)i.tagName==="LINK"&&i.rel==="modulepreload"&&o(i)}).observe(document,{childList:!0,subtree:!0});function n(s){const r={};return s.integrity&&(r.integrity=s.integrity),s.referrerPolicy&&(r.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?r.credentials="include":s.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function o(s){if(s.ep)return;s.ep=!0;const r=n(s);fetch(s.href,r)}})();const V="2026-10-30",a={appData:null,currentId:null,previousTab:"week",currentCalYear:2026,currentCalMonth:11,isFallback:!1,selectedFlyerIdx:0};async function Q(){try{localStorage.removeItem("taite_visitor_id"),localStorage.removeItem("taite_visit_count"),localStorage.removeItem("taite_telemetry_logs"),localStorage.removeItem("taite_telemetry_endpoint")}catch{}let t=!1,e=null;try{let n=await fetch("schedule.json?t="+Date.now()).catch(()=>null);if((!n||!n.ok)&&(n=await fetch("mock_schedule.json?t="+Date.now()).catch(()=>null)),n&&n.ok){e=await n.json();try{localStorage.setItem("taite_cached_schedule",JSON.stringify(e))}catch{}}else throw new Error("Network response not ok")}catch(n){console.warn("Network load failed, falling back to local cache",n);try{const o=localStorage.getItem("taite_cached_schedule");o&&(e=JSON.parse(o),t=!0)}catch{}}if(!e)throw new Error("Fatal: No schedule data available");if(a.appData=e,a.isFallback=t,e.current_next_id)a.currentId=e.current_next_id;else if(e.sessions&&e.sessions.length>0){const n=e.sessions.find(o=>o.date>=V);a.currentId=n?n.id:e.sessions[0].id}return e}function ee(){const t=document.getElementById("freshness-indicator"),e=document.getElementById("freshness-text");if(!(!t||!e)){if(a.isFallback){t.classList.remove("hidden"),t.classList.add("flex"),e.textContent="オフライン保存版を表示中";return}if(a.appData&&a.appData.generated_at)try{const n=new Date(a.appData.generated_at).getTime(),s=(Date.now()-n)/(1e3*60*60);if(s>=24){const r=Math.floor(s/24);t.classList.remove("hidden"),t.classList.add("flex"),e.textContent=`${r}日前の情報です`}else t.classList.add("hidden"),t.classList.remove("flex")}catch{t.classList.add("hidden"),t.classList.remove("flex")}}}/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const U={xmlns:"http://www.w3.org/2000/svg",width:24,height:24,viewBox:"0 0 24 24",fill:"none",stroke:"currentColor","stroke-width":2,"stroke-linecap":"round","stroke-linejoin":"round"};/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const G=([t,e,n])=>{const o=document.createElementNS("http://www.w3.org/2000/svg",t);return Object.keys(e).forEach(s=>{o.setAttribute(s,String(e[s]))}),n!=null&&n.length&&n.forEach(s=>{const r=G(s);o.appendChild(r)}),o},te=(t,e={})=>{const o={...U,...e};return G(["svg",o,t])};/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ne=(...t)=>t.filter((e,n,o)=>!!e&&e.trim()!==""&&o.indexOf(e)===n).join(" ").trim();/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const oe=t=>{for(const e in t)if(e.startsWith("aria-")||e==="role"||e==="title")return!0;return!1};/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const se=t=>{let e="",n=!1;for(const o of t){if(o==="-"||o==="_"||o<=" "){n=e.length>0;continue}e.length===0?e+=o.toLowerCase():e+=n?o.toUpperCase():o,n=!1}return e};/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ae=t=>{const e=se(t);return e.charAt(0).toUpperCase()+e.slice(1)};/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const re=t=>Array.from(t.attributes).reduce((e,n)=>(e[n.name]=n.value,e),{}),Y=t=>typeof t=="string"?t:!t||!t.class?"":t.class&&typeof t.class=="string"?t.class.split(" "):t.class&&Array.isArray(t.class)?t.class:"",z=(t,{nameAttr:e,icons:n,attrs:o})=>{var f;const s=t.getAttribute(e);if(s==null)return;const r=ae(s),i=n[r];if(!i)return console.warn(`${t.outerHTML} icon name was not found in the provided icons object.`);const l=re(t),p=oe(l)?{}:{"aria-hidden":"true"},c={...U,"data-lucide":s,...p,...o,...l},d=Y(l),m=Y(o),u=ne("lucide",`lucide-${s}`,...d,...m);u&&Object.assign(c,{class:u});const x=te(i,c);return(f=t.parentNode)==null?void 0:f.replaceChild(x,t)};/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ie=[["circle",{cx:"12",cy:"12",r:"10"}],["path",{d:"m14.31 8 5.74 9.94"}],["path",{d:"M9.69 8h11.48"}],["path",{d:"m7.38 12 5.74-9.94"}],["path",{d:"M9.69 16 3.95 6.06"}],["path",{d:"M14.31 16H2.83"}],["path",{d:"m16.62 12-5.74 9.94"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ce=[["path",{d:"M8 2v3"}],["path",{d:"M16 2v3"}],["rect",{x:"3",y:"3",width:"18",height:"18",rx:"2"}],["path",{d:"M3 9h18"}],["path",{d:"M8 13h.01"}],["path",{d:"M12 13h.01"}],["path",{d:"M16 13h.01"}],["path",{d:"M8 17h.01"}],["path",{d:"M12 17h.01"}],["path",{d:"M16 17h.01"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const le=[["path",{d:"M8 2v3"}],["path",{d:"M16 2v3"}],["rect",{x:"3",y:"3",width:"18",height:"18",rx:"2"}],["path",{d:"M3 9h18"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const de=[["path",{d:"M13.997 4a2 2 0 0 1 1.76 1.05l.486.9A2 2 0 0 0 18.003 7H20a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2h1.997a2 2 0 0 0 1.759-1.048l.489-.904A2 2 0 0 1 10.004 4z"}],["circle",{cx:"12",cy:"13",r:"3"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const pe=[["path",{d:"m15 18-6-6 6-6"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ue=[["path",{d:"m9 18 6-6-6-6"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const me=[["path",{d:"M15 3h6v6"}],["path",{d:"M10 14 21 3"}],["path",{d:"M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const fe=[["path",{d:"M12 7v14"}],["path",{d:"M20 11v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-8"}],["path",{d:"M7.5 7a1 1 0 0 1 0-5A4.8 8 0 0 1 12 7a4.8 8 0 0 1 4.5-5 1 1 0 0 1 0 5"}],["rect",{x:"3",y:"7",width:"18",height:"4",rx:"1"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const he=[["rect",{width:"18",height:"18",x:"3",y:"3",rx:"2",ry:"2"}],["circle",{cx:"9",cy:"9",r:"2"}],["path",{d:"m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const xe=[["path",{d:"M3 5h.01"}],["path",{d:"M3 12h.01"}],["path",{d:"M3 19h.01"}],["path",{d:"M8 5h13"}],["path",{d:"M8 12h13"}],["path",{d:"M8 19h13"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ge=[["path",{d:"M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0"}],["circle",{cx:"12",cy:"10",r:"3"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const be=[["path",{d:"M15 3h6v6"}],["path",{d:"m21 3-7 7"}],["path",{d:"m3 21 7-7"}],["path",{d:"M9 21H3v-6"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ye=[["path",{d:"M12 19v3"}],["path",{d:"M19 10v2a7 7 0 0 1-14 0v-2"}],["rect",{x:"9",y:"2",width:"6",height:"13",rx:"3"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const ve=[["polygon",{points:"3 11 22 2 13 21 11 13 3 11"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const we=[["path",{d:"M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"}],["path",{d:"M20 2v4"}],["path",{d:"M22 4h-4"}],["circle",{cx:"4",cy:"20",r:"2"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const Ee=[["path",{d:"M2 9a3 3 0 0 1 0 6v2a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-2a3 3 0 0 1 0-6V7a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2Z"}],["path",{d:"M13 5v2"}],["path",{d:"M13 17v2"}],["path",{d:"M13 11v2"}]];/**
 * @license lucide v1.64.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const J=({icons:t={},nameAttr:e="data-lucide",attrs:n={},root:o=document,inTemplates:s}={})=>{if(!Object.values(t).length)throw new Error(`Please provide an icons object.
If you want to use all the icons you can import it like:
 \`import { createIcons, icons } from 'lucide';
lucide.createIcons({icons});\``);if(typeof o>"u")throw new Error("`createIcons()` only works in a browser environment.");if(Array.from(o.querySelectorAll(`[${e}]`)).forEach(i=>z(i,{nameAttr:e,icons:t,attrs:n})),s&&Array.from(o.querySelectorAll("template")).forEach(l=>J({icons:t,nameAttr:e,attrs:n,root:l.content,inTemplates:s})),e==="data-lucide"){const i=o.querySelectorAll("[icon-name]");i.length>0&&(console.warn("[Lucide] Some icons were found with the now deprecated icon-name attribute. These will still be replaced for backwards compatibility, but will no longer be supported in v1.0 and you should switch to data-lucide"),Array.from(i).forEach(l=>z(l,{nameAttr:"icon-name",icons:t,attrs:n})))}},Ie={Aperture:ie,Calendar:le,CalendarDays:ce,Camera:de,ChevronLeft:pe,ChevronRight:ue,ExternalLink:me,Gift:fe,Image:he,List:xe,MapPin:ge,Maximize2:be,Mic:ye,Navigation:ve,Sparkles:we,Ticket:Ee};function b(){try{J({icons:Ie})}catch(t){console.warn("Failed to refresh Lucide icons",t)}}let h=[];function S(t){if(!h[t])return;a.selectedFlyerIdx=t;const e=h[t],n=document.getElementById("hero-strip-img"),o=document.getElementById("lightbox-img");n&&(n.src=e.url),o&&(o.src=e.url);const s=document.getElementById("hero-img-count-badge");s&&h.length>1&&(s.textContent=`${t+1}/${h.length}`),Le(),b()}function Le(){const t=document.getElementById("hero-image-pills"),e=document.getElementById("lightbox-image-pills"),n=document.getElementById("hero-img-count-badge");if(!(!t||!e)){if(t.innerHTML="",e.innerHTML="",h.length<=1){t.classList.add("hidden"),e.classList.add("hidden"),n&&n.classList.add("hidden");return}t.classList.remove("hidden"),e.classList.remove("hidden"),n&&(n.classList.remove("hidden"),n.textContent=`${a.selectedFlyerIdx+1}/${h.length}`),h.forEach((o,s)=>{const r=s===a.selectedFlyerIdx,i=o.type==="timetable"?"calendar-days":"image",l=o.type==="timetable"?"タイテ表":"フライヤー",p=document.createElement("button");p.className=`flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-bold transition-all ${r?"bg-rose-500 text-stone-950 shadow-sm":"bg-stone-900 text-stone-400 hover:text-stone-200 border border-white/10"}`,p.innerHTML=`<i data-lucide="${i}" class="w-3.5 h-3.5"></i><span>${l}</span>`,p.onclick=d=>{d.stopPropagation(),S(s)},t.appendChild(p);const c=document.createElement("button");c.className=`flex items-center gap-1 px-3 py-1 rounded-full text-xs font-bold transition-all ${r?"bg-rose-500 text-stone-950 shadow":"bg-stone-800 text-stone-300 hover:text-white border border-white/20"}`,c.innerHTML=`<i data-lucide="${i}" class="w-3.5 h-3.5"></i><span>${l}</span>`,c.onclick=d=>{d.stopPropagation(),S(s)},e.appendChild(c)})}}function Ce(t){const e=document.getElementById("sameday-switcher-container"),n=document.getElementById("sameday-pills");if(!e||!n||!a.appData)return;const o=a.appData.sessions.filter(s=>s.date===t);if(o.length<=1){e.classList.add("hidden");return}e.classList.remove("hidden"),n.innerHTML="",o.forEach(s=>{const r=s.id===a.currentId,i=document.createElement("button"),l=s.session_name||s.title.slice(0,10);i.className=`px-3 py-1 rounded-xl text-xs font-bold transition-all flex items-center gap-1.5 ${r?"bg-gradient-to-r from-rose-500 to-rose-600 text-stone-950 shadow-sm":"bg-stone-900 text-stone-400 hover:text-stone-200 border border-white/10"}`,i.innerHTML=`<span>${l}</span><span class="text-[10px] font-mono opacity-80">${s.live.start_time}~</span>`,i.onclick=()=>{w(s.id)},n.appendChild(i)})}function w(t){var R,W;if(!a.appData)return;const e=a.appData.sessions.find(y=>y.id===t);if(!e)return;a.currentId=t,h=e.images&&e.images.length>0?e.images:[{type:"flyer",label:"フライヤー",url:e.image_url||""}],a.selectedFlyerIdx=0,S(0);const n=document.getElementById("live-title");n&&(n.textContent=e.title);const o=document.getElementById("live-venue-name");o&&(o.textContent=e.venue.name);const s=document.getElementById("live-date-header");s&&(s.textContent=`${e.date} (${e.day_of_week||""})`);const r=document.getElementById("live-relative-day");if(r){const y=a.appData.current_next_id||a.appData.sessions[0]&&a.appData.sessions[0].id;t===y?(r.textContent="★ 直近ライブ",r.className="text-xs font-bold text-rose-400 bg-rose-500/15 px-2 py-0.5 rounded border border-rose-500/30"):(r.textContent="",r.className="")}Ce(e.date);const i=a.appData.current_next_id||a.appData.sessions[0]&&a.appData.sessions[0].id,l=t===i,p=document.getElementById("nav-next");p&&(p.innerHTML=l?'<i data-lucide="sparkles" class="w-3.5 h-3.5"></i><span>直近ライブ</span>':'<i data-lucide="calendar" class="w-3.5 h-3.5 text-rose-400"></i><span>公演詳細</span>');const c=document.getElementById("subpage-return-bar"),d=document.getElementById("subpage-return-bottom");c&&(l?c.classList.add("hidden"):c.classList.remove("hidden")),d&&(l?d.classList.add("hidden"):d.classList.remove("hidden"));const m=document.getElementById("live-start");m&&(m.textContent=e.live.start_time);const u=document.getElementById("live-end");u&&(u.textContent=e.live.end_time||"");const x=document.getElementById("live-stage");x&&(x.textContent=e.live.stage||"ステージ");const f=document.getElementById("benefit-type-label");f&&(f.textContent=`特典会 (${e.benefit.type||"終演後"})`);const g=document.getElementById("benefit-start");g&&(g.textContent=e.benefit.start_time);const E=document.getElementById("benefit-end");E&&(E.textContent=e.benefit.end_time||"");const H=document.getElementById("benefit-booth");H&&(H.textContent=e.benefit.booth||"");const k=document.getElementById("photo-box"),j=document.getElementById("photo-status"),A=document.getElementById("photo-desc");k&&j&&A&&(e.photo&&e.photo.rule&&e.photo.rule.trim()!==""&&(e.photo.rule.includes("曲目")||e.photo.rule.includes("『"))?(k.classList.remove("hidden"),j.textContent="撮可：",A.textContent=e.photo.rule):k.classList.add("hidden"));const F=document.getElementById("bonus-desc");F&&(F.textContent=e.admission_bonus||"特になし");const _=document.getElementById("special-notes-box"),P=document.getElementById("special-notes-desc");_&&P&&(e.special_notes?(_.classList.remove("hidden"),P.textContent=e.special_notes):_.classList.add("hidden"));const B=document.getElementById("ticket-prices-box"),$=document.getElementById("ticket-prices-list");B&&$&&($.innerHTML="",e.ticket&&e.ticket.prices&&e.ticket.prices.length>0?(B.classList.remove("hidden"),e.ticket.prices.forEach(y=>{const T=document.createElement("span");T.className="px-2 py-0.5 rounded-lg bg-stone-900 border border-white/10 text-xs font-medium text-stone-200",T.innerHTML=`<span class="text-stone-400 mr-1">${y.tier}:</span><strong class="text-rose-300 font-display font-bold">${y.price}</strong>`,$.appendChild(T)})):B.classList.add("hidden"));const O=document.getElementById("venue-map-link");O&&e.venue.map_url&&(O.href=e.venue.map_url);const I=document.getElementById("ticket-link"),D=document.getElementById("ticket-btn-text");I&&D&&(I.href=((R=e.ticket)==null?void 0:R.url)||"#",!((W=e.ticket)!=null&&W.url)||e.ticket.platform==="公式アナウンス待ち"?(D.textContent="チケット告知待ち",I.className="flex items-center justify-center gap-1.5 py-2.5 rounded-xl bg-stone-800 text-stone-400 font-bold text-xs border border-white/10 transition-colors pointer-events-none"):(D.textContent="チケット購入",I.className="flex items-center justify-center gap-1.5 py-2.5 rounded-xl bg-gradient-to-r from-rose-500 to-rose-600 hover:from-rose-400 hover:to-rose-500 text-stone-950 font-bold text-xs shadow-md transition-all")),b()}const C=["next","week","month"];let M="next";function Me(){return M}function v(t){M=t;const e=document.getElementById("views-carousel"),n=C.indexOf(t);if(e&&n!==-1){const s=e.clientWidth;e.scrollTo({left:n*s,behavior:"smooth"})}const o=document.getElementById(`tab-content-${t}`);t==="month"&&o&&o.scrollTop>0&&o.scrollTo({top:0,behavior:"auto"}),K(t)}function K(t){C.forEach(e=>{const n=document.getElementById(`nav-${e}`);n&&(e===t?n.className="relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-stone-950 transition-colors flex items-center justify-center gap-1.5":n.className="relative z-10 flex-1 py-2.5 rounded-xl text-center font-bold text-stone-400 hover:text-stone-200 transition-colors flex items-center justify-center gap-1.5")})}function ke(){const t=document.getElementById("views-carousel"),e=document.getElementById("nav-indicator");t&&t.addEventListener("scroll",()=>{const n=t.clientWidth;if(!n)return;const o=t.scrollLeft/n;e&&(e.style.transform=`translateX(${o*100}%)`);const s=Math.max(0,Math.min(C.length-1,Math.round(o))),r=C[s];M!==r&&(M=r,K(r))},{passive:!0})}function N(t,e="week"){a.previousTab=e,w(t),v("next")}function _e(){const t=document.getElementById("upcoming-list");if(!t||!a.appData)return;t.innerHTML="";const e=a.appData.sessions.filter(o=>o.date>=V),n=document.getElementById("upcoming-count-badge");n&&(n.textContent=`${e.length}公演`),e.forEach(o=>{const s=document.createElement("div");s.className="soft-card rounded-2xl p-3 flex items-center gap-3.5 cursor-pointer hover:border-rose-400/40 transition-all shadow-md",s.onclick=()=>N(o.id,"week");const[,r,i]=o.date.split("-"),l=parseInt(r,10),p=parseInt(i,10);let c="text-stone-400";o.day_of_week==="土"&&(c="text-blue-400"),(o.day_of_week==="日"||o.day_of_week&&o.day_of_week.includes("祝"))&&(c="text-rose-400");const m=(a.appData?a.appData.sessions.filter(u=>u.date===o.date).length:1)>1?o.session_name&&o.session_name.includes("昼")?" [昼]":" [夜]":"";s.innerHTML=`
      <!-- カレンダー風・日付アンカーブロック -->
      <div class="w-[52px] h-[56px] rounded-xl bg-stone-950 border border-white/10 flex flex-col items-center justify-center shrink-0 shadow-inner">
        <span class="text-[10px] font-bold text-stone-400 leading-none">${l}月</span>
        <span class="text-xl font-display font-black text-white leading-tight my-0.5">${p}</span>
        <span class="text-[10px] font-bold ${c} leading-none">${o.day_of_week||""}</span>
      </div>

      <!-- イベント情報本体 -->
      <div class="flex-1 min-w-0 space-y-1">
        <div class="flex items-center justify-between text-xs">
          <span class="text-stone-300 font-semibold truncate flex items-center gap-1">
            <i data-lucide="map-pin" class="w-3 h-3 text-rose-400 shrink-0"></i>
            <span class="truncate">${o.venue.name}</span>
          </span>
          ${m?`<span class="px-1.5 py-0.2 rounded bg-rose-500/20 text-rose-300 font-bold text-[10px] shrink-0">${m}</span>`:""}
        </div>

        <div class="text-sm font-extrabold text-white leading-snug line-clamp-2">${o.title}</div>

        <div class="text-xs flex items-center justify-between pt-1 border-t border-white/5 text-stone-300">
          <span class="flex items-center gap-1">
            <span class="text-rose-400 font-bold">出番</span>
            <strong class="text-white font-display font-bold">${o.live.start_time}〜</strong>
          </span>
          <span class="flex items-center gap-1 text-stone-400">
            <span>特典会</span>
            <strong class="text-stone-200 font-display">${o.benefit.start_time}〜</strong>
          </span>
        </div>
      </div>
    `,t.appendChild(s)}),b()}function Be(t){a.currentCalMonth+=t,a.currentCalMonth>12?(a.currentCalMonth=1,a.currentCalYear++):a.currentCalMonth<1&&(a.currentCalMonth=12,a.currentCalYear--),X(a.currentCalYear,a.currentCalMonth)}function X(t,e){const n=document.getElementById("calendar-header-title");n&&(n.textContent=`${t}年 ${e}月`);const o=document.getElementById("prev-month-label"),s=document.getElementById("next-month-label");o&&(o.textContent=`${e===1?12:e-1}月`),s&&(s.textContent=`${e===12?1:e+1}月`);const r=document.getElementById("calendar-cells");if(!r)return;r.innerHTML="";const i=new Date(t,e-1,1).getDay(),l=new Date(t,e,0).getDate();for(let c=0;c<i;c++){const d=document.createElement("div");d.className="py-2.5 text-stone-700",r.appendChild(d)}for(let c=1;c<=l;c++){const d=String(e).padStart(2,"0"),m=String(c).padStart(2,"0"),u=`${t}-${d}-${m}`,x=a.appData?a.appData.sessions.filter(E=>E.date===u):[],f=x.length>0,g=document.createElement("button");g.className=`py-2 rounded-xl flex flex-col items-center justify-center transition-all ${f?"bg-rose-950/50 border-2 border-rose-500/60 text-white font-extrabold shadow-sm scale-105":"bg-stone-900/30 text-stone-500 hover:text-stone-400"}`,g.innerHTML=`
      <span class="text-xs font-display">${c}</span>
      ${f?'<span class="w-1.5 h-1.5 rounded-full mt-1 bg-rose-400 animate-pulse"></span>':'<span class="h-1.5 mt-1"></span>'}
    `,f&&(g.onclick=()=>$e(u,x)),r.appendChild(g)}const p=document.getElementById("calendar-day-detail");p&&(p.innerHTML='<div class="text-xs text-stone-400 text-center py-2">日付をタップすると詳細が表示されます</div>'),b()}function $e(t,e){const n=document.getElementById("calendar-day-detail");n&&(n.innerHTML="",e.forEach(o=>{const s=document.createElement("div");s.className="soft-card rounded-xl p-3 space-y-1.5 border-l-4 border-l-rose-500 cursor-pointer hover:border-rose-400/40 transition-all",s.onclick=()=>N(o.id,"cal");const r=e.length>1?o.session_name&&o.session_name.includes("昼")?" [昼公演]":" [夜公演]":"";s.innerHTML=`
      <div class="flex items-center justify-between text-xs">
        <span class="font-display font-bold text-rose-300">${o.date}${r}</span>
        <span class="text-stone-300 text-xs">${o.venue.name}</span>
      </div>
      <div class="text-sm font-bold text-white pt-0.5">${o.title}</div>
      <div class="text-xs font-display text-stone-300 flex justify-between pt-1.5 border-t border-white/5">
        <span>出演: <strong class="text-white font-bold">${o.live.start_time}〜</strong></span>
        <span>物販: <strong class="text-white font-bold">${o.benefit.start_time}〜</strong> (${o.benefit.type})</span>
      </div>
    `,n.appendChild(s)}),b())}function De(t="large"){const e=document.getElementById("promo-container");if(!e||!a.appData||!a.appData.promotions)return;const n=a.appData.promotions.find(o=>o.id==="promo-ayyu-birthday")||a.appData.promotions[0];n&&(t==="large"?e.innerHTML=`
      <div class="rounded-2xl overflow-hidden soft-card border-2 border-rose-500/50 bg-stone-900 shadow-xl">
        <div class="w-full h-44 overflow-hidden bg-stone-950">
          <img src="${n.image_url}" referrerpolicy="no-referrer" alt="${n.title}" class="w-full h-full object-cover object-center">
        </div>

        <div class="p-3.5 space-y-2.5 bg-stone-900">
          <div class="flex items-center justify-between">
            <span class="px-2.5 py-0.5 rounded-md text-xs font-black bg-red-600 text-white shadow">🎂 生誕祭クラファン</span>
            <span class="text-xs font-bold text-rose-300">受付期間：〜11/22まで</span>
          </div>

          <div class="space-y-1">
            <h3 class="text-base font-extrabold text-white">
              <span class="text-rose-400 font-bold">神楽あゆ 生誕祭2026 ❤️</span>
            </h3>
            <p class="text-xs text-stone-300 leading-relaxed">初めての生誕祭＆20歳の大切な節目🎀 皆さまと素敵な1日をつくるためのクラファン開催中！</p>
          </div>

          <a href="${n.cta_url}" target="_blank" rel="noopener" class="flex items-center justify-center gap-2 w-full py-2.5 rounded-xl bg-gradient-to-r from-red-600 to-rose-600 hover:from-red-500 hover:to-rose-500 text-white font-black text-xs shadow-lg shadow-rose-950/50 transition-all">
            <span>CAMPFIREで支援する</span>
            <i data-lucide="external-link" class="w-3.5 h-3.5"></i>
          </a>
        </div>
      </div>
    `:t==="medium"?e.innerHTML=`
      <div class="rounded-2xl p-3 soft-card border border-rose-500/40 bg-stone-900 shadow-md">
        <div class="flex items-center gap-3">
          <img src="${n.image_url}" referrerpolicy="no-referrer" alt="${n.title}" class="w-16 h-16 rounded-xl object-cover shrink-0 border border-rose-400/30 shadow">
          <div class="flex-1 min-w-0 space-y-1">
            <div class="flex items-center justify-between">
              <span class="px-2 py-0.5 rounded text-[11px] font-bold bg-red-600 text-white">🎂 生誕祭クラファン</span>
              <span class="text-[11px] font-medium text-rose-300">〜11/22</span>
            </div>
            <h3 class="text-xs font-bold text-white truncate">神楽あゆ 生誕祭2026 ❤️</h3>
            <a href="${n.cta_url}" target="_blank" rel="noopener" class="inline-flex items-center gap-1 text-xs font-bold text-rose-400 hover:underline">
              <span>CAMPFIREで支援 ↗</span>
            </a>
          </div>
        </div>
      </div>
    `:e.innerHTML=`
      <div class="rounded-xl px-3 py-2 soft-card border border-rose-500/30 bg-rose-950/20 flex items-center justify-between text-xs">
        <a href="${n.cta_url}" target="_blank" rel="noopener" class="flex items-center gap-2 truncate flex-1 hover:underline">
          <span class="text-sm">🎂</span>
          <span class="font-bold text-white truncate">神楽あゆ生誕クラファン開催中 (〜11/22)</span>
          <span class="text-rose-400 shrink-0 font-bold ml-1">支援 ↗</span>
        </a>
      </div>
    `,b())}function Te(){const t=document.getElementById("lightbox-modal");t&&t.classList.add("active")}function L(){const t=document.getElementById("lightbox-modal");t&&t.classList.remove("active")}function Se(){const t=document.getElementById("lightbox-modal");if(!t)return;t.addEventListener("click",n=>{n.target===t&&L()});const e=document.getElementById("lightbox-close-btn");e&&e.addEventListener("click",n=>{n.stopPropagation(),L()}),window.addEventListener("keydown",n=>{n.key==="Escape"&&L()})}function Ne(){const t=document.getElementById("logo-icon");t&&(t.classList.remove("animate-ribbon-pop"),t.offsetWidth,t.classList.add("animate-ribbon-pop"));const e=document.getElementById("tab-content-next");e&&(e.classList.remove("animate-card-refresh"),e.offsetWidth,e.classList.add("animate-card-refresh")),Z()}function Z(){if(!a.appData)return;const t=a.appData.current_next_id||a.appData.sessions[0]&&a.appData.sessions[0].id;if(!t)return;w(t);const e=document.getElementById("tab-content-next");e&&e.scrollTo({top:0,behavior:"smooth"}),v("next")}function He(){const t=a.previousTab==="cal"?"month":"week";v(t)}function je(){if(!a.appData)return;const t=a.appData.current_next_id||a.appData.sessions[0]&&a.appData.sessions[0].id,e=document.getElementById("tab-content-next");if(Me()==="next"&&a.currentId!==t){t&&w(t),e&&e.scrollTo({top:0,behavior:"smooth"});return}v("next")}async function q(){try{await Q(),ee(),a.currentId&&w(a.currentId),_e(),X(a.currentCalYear,a.currentCalMonth),De("large"),ke(),Se(),b(),v("next")}catch(t){console.error("Failed to initialize Ribbon Clock app",t)}}window.handleLogoClick=Ne;window.goToNextLive=Z;window.goBackToList=He;window.handleFirstTabClick=je;window.changeCalendarMonth=Be;window.setTab=v;window.openLightbox=Te;window.closeLightbox=L;window.openSessionDetail=N;document.readyState==="loading"?document.addEventListener("DOMContentLoaded",()=>{q()}):q();

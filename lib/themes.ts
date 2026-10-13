export const colorKeys=['bg','surface','surface-hover','raised','text','muted','subtle','border','track','chart-primary','chart-secondary','chart-tertiary'] as const;
export type ThemeColor=typeof colorKeys[number];
export type ThemeDefinition={schemaVersion:1;id:string;name:string;mode:'dark'|'light';radius:number;colors:Record<ThemeColor,string>;accent?:string;options?:ThemeOptions};
export type ThemeOptions={font:"system"|"arial"|"segoe";contentFont:"inherit"|"system"|"arial"|"segoe";codeFont:"system"|"consolas"|"courier";fontSize:number;codeSize:number;italic:boolean;transparent:boolean;contrast:number;pointer:boolean};
export const defaultOptions:ThemeOptions={font:"system",contentFont:"inherit",codeFont:"system",fontSize:14,codeSize:13,italic:false,transparent:false,contrast:60,pointer:true};
export const builtinThemes:ThemeDefinition[]=[
 {schemaVersion:1,id:'dark',name:'Тёмная',mode:'dark',radius:10,colors:{bg:'#0b0b0b',surface:'#101010','surface-hover':'#171717',raised:'#202020',text:'#f5f5f5',muted:'#999999',subtle:'#777777',border:'#292929',track:'#252525','chart-primary':'#f5f5f5','chart-secondary':'#999999','chart-tertiary':'#555555'}},
 {schemaVersion:1,id:'light',name:'Светлая',mode:'light',radius:10,colors:{bg:'#ffffff',surface:'#ffffff','surface-hover':'#f5f5f5',raised:'#ededed',text:'#101010',muted:'#656565',subtle:'#777777',border:'#dedede',track:'#e9e9e9','chart-primary':'#151515','chart-secondary':'#797979','chart-tertiary':'#c6c6c6'}}
];
builtinThemes.push({schemaVersion:1,id:'tokyo-night',name:'Tokyo Night',mode:'dark',radius:14,accent:'#3d59a1',colors:{bg:'#1a1b26',surface:'#20212d','surface-hover':'#292b3d',raised:'#303349',text:'#a9b1d6',muted:'#8c94b7',subtle:'#777f9f',border:'#303346',track:'#292b3d','chart-primary':'#7aa2f7','chart-secondary':'#bb9af7','chart-tertiary':'#73daca'}});
export function validateTheme(input:unknown):ThemeDefinition{
 if(!input||typeof input!=='object')throw new Error('Некорректная тема');
 const v=input as ThemeDefinition;
 if(v.schemaVersion!==1||!/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(v.id)||typeof v.name!=='string'||!v.name.trim()||v.name.length>64||!['dark','light'].includes(v.mode)||!Number.isFinite(v.radius)||v.radius<0||v.radius>24||!v.colors||Object.keys(v.colors).length!==colorKeys.length)throw new Error('Некорректная тема');
 if(v.accent!==undefined&&!/^#[\da-f]{6}$/i.test(v.accent))throw new Error('Некорректный акцентный цвет');
 const o={...defaultOptions,...v.options};
 if(!['system','arial','segoe'].includes(o.font)||!['inherit','system','arial','segoe'].includes(o.contentFont)||!['system','consolas','courier'].includes(o.codeFont)||!Number.isInteger(o.fontSize)||o.fontSize<12||o.fontSize>20||!Number.isInteger(o.codeSize)||o.codeSize<10||o.codeSize>20||!Number.isFinite(o.contrast)||o.contrast<0||o.contrast>100||['italic','transparent','pointer'].some(k=>typeof o[k as keyof ThemeOptions]!=='boolean'))throw new Error('Некорректные настройки шрифта');
 const colors={} as Record<ThemeColor,string>;
 for(const key of colorKeys){if(typeof v.colors[key]!=='string'||!/^#[\da-f]{6}$/i.test(v.colors[key]))throw new Error('Цвет должен иметь формат #RRGGBB');colors[key]=v.colors[key];}
 return {schemaVersion:1,id:v.id,name:v.name,mode:v.mode,radius:v.radius,colors,accent:v.accent??colors.text,options:o};
}
export function contrastRatio(a:string,b:string){
 const lum=(hex:string)=>{const c=[1,3,5].map(i=>parseInt(hex.slice(i,i+2),16)/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4);return c[0]*.2126+c[1]*.7152+c[2]*.0722;};
 const x=lum(a),y=lum(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);
}
const fonts={system:"Arial, 'Segoe UI', sans-serif",arial:"Arial, sans-serif",segoe:"'Segoe UI', sans-serif"};
const codeFonts={system:"'Cascadia Code', Consolas, monospace",consolas:"Consolas, monospace",courier:"'Courier New', monospace"};
export function applyTheme(theme:ThemeDefinition){
 const root=document.documentElement,o={...defaultOptions,...theme.options};root.dataset.theme=theme.mode;
 for(const key of colorKeys)root.style.setProperty(`--${key}`,theme.colors[key]);
 root.style.setProperty('--radius',`${theme.radius}px`);root.style.setProperty('--accent',theme.accent??theme.colors.text);
 root.style.setProperty('--ui-font',fonts[o.font]);root.style.setProperty('--content-font',o.contentFont==='inherit'?fonts[o.font]:fonts[o.contentFont]);root.style.setProperty('--code-font',codeFonts[o.codeFont]);
 root.style.setProperty('--ui-size',`${o.fontSize}px`);root.style.setProperty('--code-size',`${o.codeSize}px`);root.style.setProperty('--ui-style',o.italic?'italic':'normal');root.style.setProperty('--contrast',`${30+o.contrast*.7}%`);
 root.dataset.sidebar=o.transparent?'transparent':'solid';root.dataset.pointer=o.pointer?'pointer':'default';
}

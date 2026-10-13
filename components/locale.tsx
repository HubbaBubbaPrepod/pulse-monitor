"use client";
import {useSyncExternalStore,type ReactNode} from 'react';
import {translate,currentLocale,defaultLocale} from '@/lib/i18n';
function subscribe(update:()=>void){window.addEventListener('pulse:locale',update);const observer=new MutationObserver(update);observer.observe(document.documentElement,{attributes:true,attributeFilter:['lang']});return()=>{observer.disconnect();window.removeEventListener('pulse:locale',update);};}
export function useLocale(){return useSyncExternalStore(subscribe,currentLocale,()=>defaultLocale);}
export function tr(key:string,params?:Record<string,unknown>){return translate(key,currentLocale(),params);}
export function trText(value:ReactNode):ReactNode{return typeof value==='string'?tr(value):Array.isArray(value)?value.map(trText):value;}

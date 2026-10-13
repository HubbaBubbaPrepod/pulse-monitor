import test from 'node:test';
import assert from 'node:assert/strict';
import {validateTheme,builtinThemes} from './themes.ts';
test('theme import preserves edited palette and rejects executable CSS',()=>{
 const theme={...builtinThemes[0],id:'custom',name:'My theme',colors:{...builtinThemes[0].colors,bg:'#112233'}};
 assert.equal(validateTheme(JSON.parse(JSON.stringify(theme))).colors.bg,'#112233');
 for(const patch of [{schemaVersion:2},{colors:{...theme.colors,bg:'url(javascript:alert(1))'}},{radius:100},{id:'../theme'}])assert.throws(()=>validateTheme({...theme,...patch}));
});

test('theme typography is bounded and has safe defaults',()=>{
 const theme=validateTheme({...builtinThemes[2],id:'custom'});
 assert.equal(theme.options.fontSize,14);
 assert.equal(theme.options.pointer,true);
 assert.equal(theme.accent,'#3d59a1');
 for(const options of [{font:'url(evil)'},{fontSize:99},{codeSize:0},{pointer:'yes'},{contrast:101}])assert.throws(()=>validateTheme({...theme,options:{...theme.options,...options}}));
 assert.throws(()=>validateTheme({...theme,id:'_theme'}));
});

test('theme names preserve spaces while being edited',()=>{
 assert.equal(validateTheme({...builtinThemes[0],name:'My '}).name,'My ');
 assert.throws(()=>validateTheme({...builtinThemes[0],name:'   '}));
});

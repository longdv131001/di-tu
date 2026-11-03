import * as THREE from 'https://cdn.jsdelivr.net/npm/three@0.160.1/build/three.module.js';

// Third-person, walkable prison world. Each named area is a real room/yard in the same scene.
export class Scene {
  constructor(canvas, onZone) {
    this.canvas = canvas; this.onZone = onZone;
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: 'high-performance' });
    this.renderer.setPixelRatio(Math.min(devicePixelRatio || 1, 1.5));
    this.renderer.shadowMap.enabled = true; this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    this.renderer.outputColorSpace = THREE.SRGBColorSpace;
    this.scene = new THREE.Scene(); this.scene.background = new THREE.Color(0xa9cbd5); this.scene.fog = new THREE.Fog(0xa9cbd5, 27, 58);
    this.camera = new THREE.PerspectiveCamera(58, 1, .1, 100); this.camera.position.set(0, 7, 10);
    this.clock = new THREE.Clock(); this.keys = {}; this.targets = []; this.colliders = []; this.zoneRects = {}; this.remote = new Map(); this.shared = {}; this.geometries = {};
    this.speed = 5.2; this.currentZone = 'yard'; this.lastZone = ''; this.low = false; this.running = true;
    this.addLights(); this.buildWorld(); this.buildPlayer(); this.bindControls(); this.resize();
    this.ro = new ResizeObserver(() => this.resize()); this.ro.observe(canvas.parentElement);
    this.animate = this.animate.bind(this); requestAnimationFrame(this.animate);
  }
  mat(color, roughness=.9) { const k = `${color}:${roughness}`; return this.shared[k] ||= new THREE.MeshStandardMaterial({ color, roughness, flatShading: true }); }
  box(x,y,z,w,h,d,color,opts={}) {
    const mesh = new THREE.Mesh(new THREE.BoxGeometry(w,h,d), this.mat(color)); mesh.position.set(x,y,z); mesh.castShadow = opts.shadow !== false; mesh.receiveShadow = true;
    this.scene.add(mesh); if (opts.solid) this.colliders.push({x,z,w,d}); if (opts.zone) { mesh.userData.zone = opts.zone; this.targets.push(mesh); }
    return mesh;
  }
  addLights() {
    this.scene.add(new THREE.HemisphereLight(0xeaf7ff, 0x69745e, 1.45));
    const sun = new THREE.DirectionalLight(0xfff1d8, 2.1); sun.position.set(-9,18,10); sun.castShadow = true; sun.shadow.mapSize.set(1024,1024); sun.shadow.camera.left=-24; sun.shadow.camera.right=24; sun.shadow.camera.top=24; sun.shadow.camera.bottom=-24; this.scene.add(sun);
  }
  floor(x,z,w,d,color=0xb8b6a4) { this.box(x,-.13,z,w,.25,d,color); }
  wall(x,z,w,d,h=3,color=0xd9d3c5) { this.box(x,h/2,z,w,h,d,color,{solid:true}); }
  room(name,cx,cz,w,d,color,furnish) {
    this.floor(cx,cz,w,d,color);
    // Three walls leave a generous doorway on the south side so the player can enter.
    this.wall(cx-w/2,cz,w? .22:.22,d,3); this.wall(cx+w/2,cz,.22,d,3);
    this.wall(cx,cz-d/2,w,.22,3); this.wall(cx-w*.29,cz+d/2,w*.42,.22,3); this.wall(cx+w*.29,cz+d/2,w*.42,.22,3);
    this.zoneRects[name] = {x:cx,z:cz,w:w-.8,d:d-.8};
    this.box(cx,0,cz,w,.04,d,0xffffff,{zone:name,shadow:false});
    furnish?.(cx,cz,w,d);
    // sign
    this.box(cx,2.15,cz-d/2-.16,Math.min(2.5,w*.7),.48,.12,0x344b54);
  }
  buildWorld() {
    this.floor(0,0,42,34,0x9ba98e);
    // perimeter fence and watch towers
    for(let x=-20;x<=20;x+=1.2){this.box(x,1.05,-16.5,.07,2.1,.07,0x52636a);this.box(x,1.05,16.5,.07,2.1,.07,0x52636a)}
    for(let z=-16.5;z<=16.5;z+=1.2){this.box(-20,1.05,z,.07,2.1,.07,0x52636a);this.box(20,1.05,z,.07,2.1,.07,0x52636a)}
    // central yard with paths
    this.floor(0,1,11,10,0x829b70); this.box(0,.02,1,10.7,.08,9.7,0x8eaa79);
    for(let i=-4;i<=4;i+=2)this.box(i,.08,1,.08,.02,9,0xd9d5b7,{shadow:false});
    // cell block, interior beds and lockers
    this.room('cell',-10,-7,8,7,0xb7aa91,(x,z)=>{
      for(let i=0;i<3;i++){const bx=x-2.5+i*2.5;this.box(bx,.35,z-1.5,1.65,.7,2,0x657f91,{solid:true});this.box(bx,.82,z-1.5,1.7,.12,2.05,0xd2c6ae);this.box(bx,.65,z+1.7,.65,1.3,.65,0x786c59,{solid:true});}
      this.box(x,.05,z+2.8,7,.08,.08,0x4e5a5c);
    });
    // workshop interior
    this.room('workshop',0,-9,8,7,0xaaa58f,(x,z)=>{for(let i=-1;i<=1;i++){this.box(x+i*2,.55,z-1,1.45,1.1,1.2,0x8b6245,{solid:true});this.box(x+i*2,.18,z+1.1,1.8,.36,.8,0x6f7d80,{solid:true});}});
    // canteen interior
    this.room('canteen',10,-7,8,7,0xc8b99d,(x,z)=>{for(let i=0;i<2;i++)for(let j=0;j<2;j++){this.box(x-2.1+i*4,.42,z-1.3+j*2.6,2.5,.18,.85,0x8a6748,{solid:true});for(const dx of [-.9,.9])this.box(x-2.1+i*4+dx,.2,z-1.3+j*2.6,.12,.4,.12,0x59656b);}});
    // gym
    this.room('gym',-10,6,8,7,0x9caa9c,(x,z)=>{for(let i=-1;i<=1;i++){this.box(x+i*2,.35,z-1,1.3,.7,.45,0x52636b,{solid:true});this.box(x+i*2,.8,z-1,.18,.18,1.6,0x59656b);}});
    // classroom
    this.room('classroom',0,9,8,7,0xc8c1a8,(x,z)=>{for(let r=0;r<2;r++)for(let i=-1;i<=1;i++)this.box(x+i*2,.42,z-1+r*2,1.25,.18,.75,0x8c674a,{solid:true});this.box(x,1.35,z-2.8,3,.08,.12,0x41645c);});
    // visiting / recreation zone
    this.room('visiting',10,7,8,7,0xb9b39e,(x,z)=>{for(let i=-1;i<=1;i++){this.box(x+i*2,.42,z,1.2,.16,1.1,0x8a6748,{solid:true});this.box(x+i*2,.9,z,.08,.8,.08,0x66747b);}});
    // gatehouse and open entrance
    this.floor(0,15,12,3,0x777f7d); this.box(-5,1.5,15,.35,3,.35,0x39474b,{solid:true}); this.box(5,1.5,15,.35,3,.35,0x39474b,{solid:true}); this.box(0,2.9,15,10,.25,.35,0x39474b);
    this.zoneRects.yard={x:0,z:1,w:5,d:4}; this.zoneRects.gate={x:0,z:14,w:3,d:1.5};
    this.box(0,.03,1,10,.05,9,0xffffff,{zone:'yard',shadow:false}); this.box(0,.03,14,10,.05,2,0xffffff,{zone:'gate',shadow:false});
    // decorative trees around the yard
    for(const [x,z] of [[-17,-12],[17,-12],[-17,12],[17,12],[-5,-14],[5,-14]]){this.box(x,.7,z,.35,1.4,.35,0x795b43);const crown=new THREE.Mesh(new THREE.ConeGeometry(1.15,2.5,6),this.mat(0x4e8059));crown.position.set(x,2.5,z);crown.castShadow=true;this.scene.add(crown)}
  }
  makeAvatar(name,color) {
    const g=new THREE.Group();const body=new THREE.Mesh(new THREE.CapsuleGeometry(.32,.55,4,8),this.mat(color));body.position.y=.72;body.castShadow=true;g.add(body);
    const head=new THREE.Mesh(new THREE.SphereGeometry(.23,12,8),this.mat(0xe7bd9b));head.position.y=1.42;g.add(head);
    const label=this.label(name);label.position.y=2.05;g.add(label);return g;
  }
  buildPlayer(){this.player=this.makeAvatar('Bạn',0xf28c45);this.scene.add(this.player);this.player.position.set(-10,0,-2.7);this.player.rotation.y=Math.PI;this.velocity=new THREE.Vector3();this.destination=null;this.cameraTarget=new THREE.Vector3();}
  label(text){const c=document.createElement('canvas');c.width=256;c.height=64;const ctx=c.getContext('2d');ctx.fillStyle='#ffffff';ctx.font='bold 30px sans-serif';ctx.textAlign='center';ctx.fillText(text,128,42);const tex=new THREE.CanvasTexture(c);const s=new THREE.Sprite(new THREE.SpriteMaterial({map:tex,transparent:true,depthTest:false}));s.scale.set(1.6,.4,1);return s;}
  bindControls(){
    window.addEventListener('keydown',e=>{const k=e.key.toLowerCase();if(['w','a','s','d','arrowup','arrowdown','arrowleft','arrowright',' '].includes(k)&&!['INPUT','TEXTAREA'].includes(document.activeElement?.tagName)){this.keys[k]=true;e.preventDefault();}});
    window.addEventListener('keyup',e=>{this.keys[e.key.toLowerCase()]=false;});
    this.canvas.addEventListener('click',e=>{if(e.detail===2)this.canvas.requestPointerLock?.();else this.pick(e)});
    document.addEventListener('mousemove',e=>{if(document.pointerLockElement===this.canvas)this.player.rotation.y-=e.movementX*.0025;});
    this.canvas.addEventListener('touchstart',e=>{if(e.touches.length){this.touchStart={x:e.touches[0].clientX,y:e.touches[0].clientY};}},{passive:true});
    this.canvas.addEventListener('touchend',e=>{if(!this.touchStart||!e.changedTouches.length)return;const t=e.changedTouches[0],dx=t.clientX-this.touchStart.x,dy=t.clientY-this.touchStart.y;if(Math.hypot(dx,dy)>18){this.touchVector={x:THREE.MathUtils.clamp(dx/55,-1,1),y:THREE.MathUtils.clamp(dy/55,-1,1)};setTimeout(()=>this.touchVector=null,180)}this.touchStart=null;},{passive:true});
  }
  blocked(x,z){return this.colliders.some(c=>Math.abs(x-c.x)<c.w/2+.38&&Math.abs(z-c.z)<c.d/2+.38);}
  moveTo(zone){const r=this.zoneRects[zone];if(!r)return;this.currentZone=zone;const isRoom=!['yard','gate'].includes(zone);this.route=isRoom?[new THREE.Vector3(r.x,0,r.z+r.d/2+1.05),new THREE.Vector3(r.x,0,r.z)]:[new THREE.Vector3(r.x,0,r.z)];this.destination=this.route.shift();this.onZone(zone);}
  pick(e){const rect=this.canvas.getBoundingClientRect(),mouse=new THREE.Vector2((e.clientX-rect.left)/rect.width*2-1,-(e.clientY-rect.top)/rect.height*2+1);const ray=new THREE.Raycaster();ray.setFromCamera(mouse,this.camera);const hit=ray.intersectObjects(this.targets,false)[0];if(hit){const zone=hit.object.userData.zone;this.moveTo(zone);}}
  setPlayers(data){if(!data)return;const me=data.me?.id;const keep=new Set();for(const p of data.players||[]){if(!p.id||p.id===me)continue;keep.add(p.id);let a=this.remote.get(p.id);if(!a){a=this.makeAvatar(p.name||'Bạn tù',p.color||0xf28c45);this.remote.set(p.id,a);this.scene.add(a)}const r=this.zoneRects[p.zone]||this.zoneRects.yard;const hash=[...p.id].reduce((v,c)=>(v*31+c.charCodeAt(0))|0,0);a.userData.target=new THREE.Vector3(r.x+(Math.abs(hash)%5-2)*.55,0,r.z+(Math.abs(hash>>3)%3-1)*.55);}for(const [id,a] of this.remote)if(!keep.has(id)){this.scene.remove(a);this.remote.delete(id)}}
  setColor(c){this.player.children[0].material.color.set(c)} setHair(style){/* hairstyle customization can be extended without changing movement */}
  setLow(v){this.low=v;this.renderer.setPixelRatio(v?Math.min(devicePixelRatio||1,.8):Math.min(devicePixelRatio||1,1.5));this.renderer.shadowMap.enabled=!v;this.resize()}
  resize(){const w=this.canvas.clientWidth||600,h=this.canvas.clientHeight||400;this.renderer.setSize(w,h,false);this.camera.aspect=w/h;this.camera.updateProjectionMatrix()}
  resume(){this.running=true;this.clock.getDelta();requestAnimationFrame(this.animate)}
  update(dt){const forward=(this.keys.w||this.keys.arrowup?1:0)-(this.keys.s||this.keys.arrowdown?1:0)+(this.touchVector?-this.touchVector.y:0);const side=(this.keys.d||this.keys.arrowright?1:0)-(this.keys.a||this.keys.arrowleft?1:0)+(this.touchVector?this.touchVector.x:0);
    const moving=Math.abs(forward)+Math.abs(side)>.01; if(moving){this.destination=null;const dir=new THREE.Vector3(side,0,-forward).normalize().applyAxisAngle(new THREE.Vector3(0,1,0),this.camera.rotation.y);const nx=this.player.position.x+dir.x*this.speed*dt,nz=this.player.position.z+dir.z*this.speed*dt;if(!this.blocked(nx,this.player.position.z))this.player.position.x=THREE.MathUtils.clamp(nx,-19,19);if(!this.blocked(this.player.position.x,nz))this.player.position.z=THREE.MathUtils.clamp(nz,-15.5,15.5);this.player.rotation.y=THREE.MathUtils.damp(this.player.rotation.y,Math.atan2(dir.x,dir.z),10,dt);}
    else if(this.destination){const delta=this.destination.clone().sub(this.player.position);if(delta.length()<.22){this.destination=this.route?.shift()||null;}else{const step=Math.min(delta.length(),this.speed*dt);const nx=this.player.position.x+delta.x/delta.length()*step,nz=this.player.position.z+delta.z/delta.length()*step;if(!this.blocked(nx,nz))this.player.position.set(nx,0,nz);else this.destination=this.route?.shift()||null;}}
    const desired=new THREE.Vector3(this.player.position.x,6.2,this.player.position.z+8.5);this.camera.position.lerp(desired,1-Math.exp(-5*dt));this.camera.lookAt(this.player.position.x,1,this.player.position.z-1.5);
    for(const a of this.remote.values())if(a.userData.target)a.position.lerp(a.userData.target,1-Math.exp(-5*dt));
  }
  animate(){if(!this.running)return;requestAnimationFrame(this.animate);if(document.hidden)return;const dt=Math.min(this.clock.getDelta(),.05);this.update(dt);this.renderer.render(this.scene,this.camera)}
}

import { isIP } from 'node:net';
import { Problem } from './security.mjs';

// Aggregate IPv6 clients by /64 so rotating addresses cannot reset the budget.
export function clientNetwork(address) {
  if(isIP(address)===4)return address;
  if(isIP(address)!==6)return 'unknown';
  let value=address.toLowerCase();
  if(value.includes('.')) {
    const tail=value.slice(value.lastIndexOf(':')+1).split('.').map(Number);
    value=value.slice(0,value.lastIndexOf(':')+1)+((tail[0]<<8)|tail[1]).toString(16)+':'+((tail[2]<<8)|tail[3]).toString(16);
  }
  const [left,right]=value.split('::'),a=left?left.split(':'):[],b=right?right.split(':'):[];
  const words=(right!==undefined?[...a,...Array(8-a.length-b.length).fill('0'),...b]:a).map(v=>parseInt(v,16));
  if(words.slice(0,5).every(v=>v===0)&&words[5]===65535)return `${words[6]>>8}.${words[6]&255}.${words[7]>>8}.${words[7]&255}`;
  return words.slice(0,4).map(v=>v.toString(16)).join(':')+'::/64';
}

export function abuseProtection({now=Date.now,maxBuckets=20000}={}) {
  const buckets=new Map(),pending=new Map();let total=0,lastSweep=0;
  const denied=()=>new Problem('RATE_LIMITED','操作较频繁，请稍后再试。',429);
  const hit=(key,max=120,cost=1,window=60000)=>{
    const at=now();
    if(at-lastSweep>=60000){for(const [k,b] of buckets)if(at>=b.expires)buckets.delete(k);lastSweep=at;}
    let b=buckets.get(key);
    if(!b||at>=b.expires){if(!b&&buckets.size>=maxBuckets)throw denied();b={count:0,expires:at+window};buckets.set(key,b);}
    b.count+=cost;
    if(b.count>max)throw denied();
  };
  return {
    hit,
    discovery(key){const b=buckets.get('invalid:'+key);if(b&&now()<b.expires&&b.count>=30)throw denied();},
    invalid(key,cost=1){hit('invalid:'+key,30,cost);},
    acquire(key){
      if(total>=32||(pending.get(key)||0)>=6)throw denied();
      total++;pending.set(key,(pending.get(key)||0)+1);let released=false;
      return ()=>{if(released)return;released=true;total--;const n=pending.get(key)-1;if(n)pending.set(key,n);else pending.delete(key);};
    },
  };
}

// Isolated cross-process shop integration fixture. This file is never shipped.
import { randomBytes } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import { Store } from '../server/store.mjs';
import { vault } from '../server/security.mjs';
import { MockPartner } from '../server/mock.mjs';
import { RedemptionService } from '../server/service.mjs';
import { makeServer } from '../server/http.mjs';

const store=new Store(process.env.PAIPLUS_FIXTURE_DB);
const partner=new MockPartner(store);
for(const source of JSON.parse(process.env.PAIPLUS_FIXTURE_SOURCES))partner.seed(source);
const service=new RedemptionService({store,vault:vault(process.env.PAIPLUS_FIXTURE_KEY||randomBytes(32).toString('base64')),partner,publicOrigin:process.env.PAIPLUS_FIXTURE_ORIGIN,mock:true});
const snapshot=()=>writeFileSync(process.env.PAIPLUS_FIXTURE_STATS,JSON.stringify({cards:store.get('SELECT count(*) AS n FROM cards').n,deliveries:store.get('SELECT count(*) AS n FROM deliveries').n,submits:partner.calls.filter(c=>c.operation==='redeem').length}));
const deliver=service.deliver.bind(service);
service.deliver=input=>{const result=deliver(input);snapshot();return result;};
const call=partner.call.bind(partner);
partner.call=async(...args)=>{try{return await call(...args);}finally{snapshot();}};
const server=makeServer({service,config:{mode:'mock',publicOrigin:process.env.PAIPLUS_FIXTURE_ORIGIN,adminToken:'fixture-admin-token-not-a-production-secret',integrationToken:'fixture-integration-token-not-a-production-secret',managementToken:'fixture-management-token-not-a-production-secret',examples:[]},staticDir:'.'});
server.listen(0,'127.0.0.1',()=>console.log(`http://127.0.0.1:${server.address().port}`));

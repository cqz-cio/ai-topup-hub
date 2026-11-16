// Local fixtures implement only the documented create/status contract. They
// cannot contact the provider or charge an account.
export class MockWudiPartner {
  constructor(store) {this.store=store;this.key='wpk_local_mock_partner_not_a_real_secret';this.calls=[];}
  async create(code,requestID) {
    this.calls.push({operation:'redemptions',code,client_request_id:requestID});
    return {id:'mock-'+requestID,client_request_id:requestID,status:'awaiting_customer',redeem_url:'https://wudi.cdk.bar/#partner=LOCAL_MOCK_DO_NOT_REDEEM',expires_at:new Date(Date.now()+86400000).toISOString(),card_reenabled:false};
  }
  async status(intentID) {
    this.calls.push({operation:'status',id:intentID});
    return {status:'awaiting_customer',card_reenabled:false};
  }
}

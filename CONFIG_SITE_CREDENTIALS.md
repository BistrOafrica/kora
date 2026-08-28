# Config UAT Sites and Credentials

Local development/UAT credentials generated on 2026-08-03. Every site has an
isolated MySQL database and administrator account. URLs use the path-based
tenant router:

`http://localhost:8000/s/{site}/workspace`

| Config | Site | Admin email | Admin password |
|---|---|---|---|
| accounting | accounting-uat.local | admin@accounting-uat.local | `KoraUAT-Accounting-2026!` |
| agriculture | agriculture-uat.local | admin@agriculture-uat.local | `KoraUAT-agriculture-2026!` |
| airtime | airtime-uat.local | admin@airtime-uat.local | `KoraUAT-airtime-2026!` |
| budgeting | budgeting-uat.local | admin@budgeting-uat.local | `KoraUAT-budgeting-2026!` |
| clinic | clinic-uat.local | admin@clinic-uat.local | `KoraUAT-clinic-2026!` |
| construction | construction-uat.local | admin@construction-uat.local | `KoraUAT-construction-2026!` |
| content | content-uat.local | admin@content-uat.local | `KoraUAT-content-2026!` |
| contracts | contracts-uat.local | admin@contracts-uat.local | `KoraUAT-contracts-2026!` |
| crm | crm-uat.local | admin@crm-uat.local | `KoraUAT-crm-2026!` |
| customer-success | customer-success-uat.local | admin@customer-success-uat.local | `KoraUAT-customer-success-2026!` |
| documents | documents-uat.local | admin@documents-uat.local | `KoraUAT-documents-2026!` |
| ecommerce | ecommerce-uat.local | admin@ecommerce-uat.local | `KoraUAT-ecommerce-2026!` |
| erp | erp-uat.local | admin@erp-uat.local | `KoraUAT-erp-2026!` |
| erp_kenya | erp_kenya-uat.local | admin@erp_kenya-uat.local | `KoraUAT-erp_kenya-2026!` |
| events | events-uat.local | admin@events-uat.local | `KoraUAT-events-2026!` |
| expense | expense-uat.local | admin@expense-uat.local | `KoraUAT-expense-2026!` |
| fieldwork | fieldwork-uat.local | admin@fieldwork-uat.local | `KoraUAT-fieldwork-2026!` |
| fleet | fleet-uat.local | admin@fleet-uat.local | `KoraUAT-fleet-2026!` |
| helpdesk | helpdesk-uat.local | admin@helpdesk-uat.local | `KoraUAT-helpdesk-2026!` |
| hotel | hotel-uat.local | admin@hotel-uat.local | `KoraUAT-hotel-2026!` |
| hr | hr-uat.local | admin@hr-uat.local | `KoraUAT-hr-2026!` |
| internal-requests | internal-requests-uat.local | admin@internal-requests-uat.local | `KoraUAT-internal-requests-2026!` |
| inventory | inventory-uat.local | admin@inventory-uat.local | `KoraUAT-inventory-2026!` |
| invoicing | invoicing-uat.local | admin@invoicing-uat.local | `KoraUAT-invoicing-2026!` |
| kiosk | kiosk-uat.local | admin@kiosk-uat.local | `KoraUAT-kiosk-2026!` |
| lms | lms-uat.local | admin@lms-uat.local | `KoraUAT-lms-2026!` |
| logistics | logistics-uat.local | admin@logistics-uat.local | `KoraUAT-logistics-2026!` |
| maintenance | maintenance-uat.local | admin@maintenance-uat.local | `KoraUAT-maintenance-2026!` |
| manufacturing | manufacturing-uat.local | admin@manufacturing-uat.local | `KoraUAT-manufacturing-2026!` |
| marketing | marketing-uat.local | admin@marketing-uat.local | `KoraUAT-marketing-2026!` |
| membership | membership-uat.local | admin@membership-uat.local | `KoraUAT-membership-2026!` |
| ngo | ngo-uat.local | admin@ngo-uat.local | `KoraUAT-ngo-2026!` |
| payroll | payroll-uat.local | admin@payroll-uat.local | `KoraUAT-payroll-2026!` |
| pharmacy | pharmacy-uat.local | admin@pharmacy-uat.local | `KoraUAT-pharmacy-2026!` |
| pos | pos-uat.local | admin@pos-uat.local | `KoraUAT-pos-2026!` |
| professional-services | professional-services-uat.local | admin@professional-services-uat.local | `KoraUAT-professional-services-2026!` |
| projectmgmt | projectmgmt-uat.local | admin@projectmgmt-uat.local | `KoraUAT-projectmgmt-2026!` |
| propertymgmt | propertymgmt-uat.local | admin@propertymgmt-uat.local | `KoraUAT-propertymgmt-2026!` |
| purchasing | purchasing-uat.local | admin@purchasing-uat.local | `KoraUAT-purchasing-2026!` |
| quality | quality-uat.local | admin@quality-uat.local | `KoraUAT-quality-2026!` |
| recruitment | recruitment-uat.local | admin@recruitment-uat.local | `KoraUAT-recruitment-2026!` |
| restaurant | restaurant-uat.local | admin@restaurant-uat.local | `KoraUAT-restaurant-2026!` |
| retail-pos | retail-pos-uat.local | admin@retail-pos-uat.local | `KoraUAT-retail-pos-2026!` |
| sacco | sacco-uat.local | admin@sacco-uat.local | `KoraUAT-sacco-2026!` |
| school | school-uat.local | admin@school-uat.local | `KoraUAT-school-2026!` |
| small-business | small-business-uat.local | admin@small-business-uat.local | `KoraUAT-small-business-2026!` |
| subscriptions | subscriptions-uat.local | admin@subscriptions-uat.local | `KoraUAT-subscriptions-2026!` |
| supermarket | supermarket-uat.local | admin@supermarket-uat.local | `KoraUAT-supermarket-2026!` |
| todo | todo-uat.local | admin@todo-uat.local | `KoraUAT-todo-2026!` |
| wholesale | wholesale-uat.local | admin@wholesale-uat.local | `KoraUAT-wholesale-2026!` |

These are local UAT credentials, not production secrets. Rotate or remove this
file before sharing the repository outside the development environment.

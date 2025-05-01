import ballerina/http;

service /customer\-data/'1\.0\.0 on new http:Listener(9090) {
    # Endpoint to get all profiles
    #
    # + return - returns can be any of following types 
    # http:InternalServerError (Server encountered error while responding to the request)
    resource function get profiles() returns Profile[]|http:InternalServerError {
        return [
            {
                profile_id: "09f5a769-9eb2-40ec-9c76-478e303724af",
                origin_country: "",
                identity_attributes: {
                    email: ["cvivekvinushanth@gmail.com"],
                    phone_number: ["+1234567890"]
                },
                traits: {
                    interests: ["Plush Toys", "Games Puzzles"],
                    spending_capability: 500
                },
                application_data: [
                    {
                        application_id: "custodian_client_app",
                        devices: [
                            {
                                device_id: "a2cdf159-502e-47e0-8ce6-922e693dfbdd",
                                last_used: 1746014018,
                                os: "macOS",
                                browser: "Chrome"
                            }
                        ]
                    }
                ]
            }
        ];
    }

    # Endpoint to retrieve a profile by ID
    #
    # + profile_id - Unique identifier for the profile
    # + return - returns can be any of following types 
    # http:NotFound (Profile not found)
    resource function get profiles/[string profile_id]() returns Profile|error {
        return {
            profile_id: profile_id,
            origin_country: "",
            identity_attributes: {
                email: ["cvivekvinushanth@gmail.com"],
                phone_number: ["+1234567890"]
            },
            traits: {
                interests: ["Plush Toys", "Games Puzzles"],
                spending_capability: 500
            },
            application_data: [
                {
                    application_id: "custodian_client_app",
                    devices: [
                        {
                            device_id: "a2cdf159-502e-47e0-8ce6-922e693dfbdd",
                            last_used: 1746014018,
                            os: "macOS",
                            browser: "Chrome"
                        }
                    ]
                }
            ]
        };
    }

    # Endpoint to delete a profile by ID
    #
    # + profile_id - Unique identifier for the profile
    # + return - returns can be any of following types 
    # http:NoContent (Profile deleted successfully)
    # http:NotFound (Profile not found)
    resource function delete profiles/[string profile_id]() returns http:NoContent|http:NotFound {
        return http:NO_CONTENT;
    }

    # Endpoint to get traits of a profile
    #
    # + profile_id - Unique identifier for the profile
    # + return - returns can be any of following types 
    # http:NotFound (Profile not found)
    resource function get profiles/[string profile_id]/traits() returns record {}|http:NotFound {
        return {
            "interests": ["Plush Toys", "Games Puzzles"],
            "spending_capability": 500
        };
    }

    # Endpoint to add a single event
    #
    # + payload - Event data
    # + return - returns can be any of following types 
    # http:Created (Event added successfully)
    # http:BadRequest (Invalid event object)
    resource function post events(@http:Payload Event payload) returns http:Created|http:BadRequest {
        return http:CREATED;
    }

    # Endpoint to get events
    #
    # + return - returns can be any of following types 
    # http:InternalServerError (Server encountered error while responding to the request)
    resource function get events() returns Event[]|http:InternalServerError {
        return [
            {
                profile_id: "02efada5-e235-42e5-b6eb-5f94c641e36d",
                applicationId: "custodian_client_app",
                org_id: "carbon.super",
                event_type: "Track",
                event_name: "category_searched",
                event_id: "3ad853cc-492c-4063-884f-e7ae5bdd9d8f",
                event_timestamp: 1746018379,
                context: {
                    user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36",
                    browser: "Chrome",
                    os: "macOS",
                    screen: { width: 1728, height: 1117 },
                    locale: "en-US",
                    timezone: "Asia/Colombo",
                    device_id: "0105b700-2efe-4567-afbe-14cdd99bb6ac"
                },
                locale: "en-US",
                properties: {
                    action: "select_category",
                    objecttype: "category",
                    objectname: "Plush Toys"
                }
            }
        ];
    }

    # Endpoint to get a specific event by ID
    #
    # + event_id - Unique identifier for the event
    # + return - returns can be any of following types 
    # http:NotFound (Event not found)
    resource function get events/[string event_id]() returns Event|error {
        return {
            profile_id: "02efada5-e235-42e5-b6eb-5f94c641e36d",
            applicationId: "custodian_client_app",
            org_id: "carbon.super",
            event_type: "Track",
            event_name: "category_searched",
            event_id: event_id,
            event_timestamp: 1746018379,
            context: {
                user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36",
                browser: "Chrome",
                os: "macOS",
                screen: { width: 1728, height: 1117 },
                locale: "en-US",
                timezone: "Asia/Colombo",
                device_id: "0105b700-2efe-4567-afbe-14cdd99bb6ac"
            },
            locale: "en-US",
            properties: {
                action: "select_category",
                objecttype: "category",
                objectname: "Plush Toys"
            }
        };
    }

    # Endpoint to get write key by application ID
    #
    # + application_id - Unique identifier for the application
    # + return - returns can be any of following types 
    # http:NotFound (Write key not found)
    resource function get events/write\-key/[string application_id]() returns string|error {
        return ""; // Replace with actual logic
    }

    # Endpoint to add a new identity resolution rule
    #
    # + payload - Unification rule data
    # + return - returns can be any of following types 
    # http:Created (Unification rule added successfully)
    # http:BadRequest (Invalid unification rule object)
    resource function post unification\-rules(@http:Payload UnificationRule payload) returns http:Created|error {
        return http:CREATED;
    }

    # Endpoint to get all identity resolution rules
    #
    # + return - returns can be any of following types 
    # http:InternalServerError (Server encountered error while responding to the request)
    resource function get unification\-rules() returns UnificationRule[]|error {
        return [
            { rule_id: "rule1", description: "Merge profiles with same email", conditions: {} },
            { rule_id: "rule2", description: "Merge profiles with same phone number", conditions: {} }
        ];
    }

    # Endpoint to get a specific unification rule by ID
    #
    # + rule_id - Unique identifier for the unification rule
    # + return - returns can be any of following types 
    # http:NotFound (Unification rule not found)
    resource function get unification\-rules/[string rule_id]() returns UnificationRule|error {
        return { rule_id: rule_id, description: "Merge profiles with same email", conditions: {} };
    }

    # Endpoint to delete a unification rule by ID
    #
    # + rule_id - Unique identifier for the unification rule
    # + return - returns can be any of following types 
    # http:NoContent (Unification rule deleted successfully)
    # http:NotFound (Unification rule not found)
    resource function delete unification\-rules/[string rule_id]() returns http:NoContent|error {
        return http:NO_CONTENT;
    }

    # Endpoint to create a profile enrichment rule
    #
    # + payload - Profile enrichment rule data
    # + return - returns can be any of following types 
    # http:Created (Profile enrichment rule added successfully)
    # http:BadRequest (Invalid profile enrichment rule object)
    resource function post enrichment\-rules(@http:Payload ProfileEnrichmentRule payload) returns http:Created|error {
        return http:CREATED;
    }

    # Endpoint to get all profile enrichment rules
    #
    # + return - returns can be any of following types 
    # http:InternalServerError (Server encountered error while responding to the request)
    resource function get enrichment\-rules() returns ProfileEnrichmentRule[]|error {
        return [
            { rule_id: "enrich1", description: "Add loyalty points", enrichment_data: {} },
            { rule_id: "enrich2", description: "Add preferred language", enrichment_data: {} }
        ];
    }

    # Endpoint to get a specific profile enrichment rule by ID
    #
    # + rule_id - Unique identifier for the profile enrichment rule
    # + return - returns can be any of following types 
    # http:NotFound (Profile enrichment rule not found)
    resource function get enrichment\-rules/[string rule_id]() returns ProfileEnrichmentRule|error {
        return { rule_id: rule_id, description: "Add loyalty points", enrichment_data: {} };
    }

    # Endpoint to replace a profile enrichment rule
    #
    # + rule_id - Unique identifier for the profile enrichment rule
    # + payload - Profile enrichment rule data
    # + return - returns can be any of following types 
    # http:Ok (Profile enrichment rule replaced successfully)
    # http:BadRequest (Invalid profile enrichment rule object)
    resource function put enrichment\-rules/[string rule_id](@http:Payload ProfileEnrichmentRule payload) returns http:Ok|error {
        return http:OK;
    }

    # Endpoint to delete a profile enrichment rule by ID
    #
    # + rule_id - Unique identifier for the profile enrichment rule
    # + return - returns can be any of following types 
    # http:NoContent (Profile enrichment rule deleted successfully)
    # http:NotFound (Profile not found)
    resource function delete enrichment\-rules/[string rule_id]() returns http:NoContent|error {
        return http:NO_CONTENT;
    }

    # Endpoint to give or update consent
    #
    # + payload - Consent data
    # + return - returns can be any of following types 
    # http:Created (Consent added successfully)
    # http:BadRequest (Invalid consent object)
    resource function post consents(@http:Payload Consent payload) returns http:Created|error {
        // Example implementation for adding a consent
        return http:CREATED;
    }

    # Endpoint to get all consents for a user
    #
    # + profile_id - Unique identifier for the profile
    # + return - returns can be any of following types 
    # http:NotFound (Profile not found)
    resource function get consents/[string profile_id]() returns Consent[]|error {
        return [
            {
                consent_id: "consent1",
                profile_id: profile_id,
                application_id: "app1",
                category_identifier: "analytics",
                granted: true,
                consent_channel: "web",
                timestamp: 1682841600,
                source_ip: "192.168.1.1",
                user_agent: "Mozilla/5.0"
            },
            {
                consent_id: "consent2",
                profile_id: profile_id,
                application_id: "app2",
                category_identifier: "marketing",
                granted: false,
                consent_channel: "mobile",
                timestamp: 1682845200,
                source_ip: "192.168.1.2",
                user_agent: "Mozilla/5.0"
            }
        ];
    }

    # Endpoint to revoke all consents for a user
    #
    # + profile_id - Unique identifier for the profile
    # + return - returns can be any of following types 
    # http:NoContent (Consents revoked successfully)
    # http:NotFound (Profile not found)
    resource function delete consents/[string profile_id]() returns http:NoContent|http:NotFound {
        // Example implementation for revoking all consents for a user
        return http:NO_CONTENT;
    }

    # Endpoint to get all consent categories
    #
    # + return - returns can be any of following types 
    # http:InternalServerError (Server encountered error while responding to the request)
    resource function get consent\-categories() returns ConsentCategory[]|error {
        return [
            {
                id: "7fa06d1e-688f-481b-8263-29c1f5ce1493",
                category_name: "User behavior analytics",
                category_identifier: "analytics",
                purpose: "profiling",
                destinations: ["dest1", "dest2"]
            },
            {
                id: "8fa06d1e-688f-481b-8263-29c1f5ce1494",
                category_name: "Marketing",
                category_identifier: "marketing",
                purpose: "advertising",
                destinations: ["dest1"]
            }
        ];
    }

    # Endpoint to add a consent category
    #
    # + payload - Consent category data
    # + return - returns can be any of following types 
    # http:Created (Consent category added successfully)
    # http:BadRequest (Invalid consent category object)
    resource function post consent\-categories(@http:Payload ConsentCategory payload) returns http:Created|error {
        return http:CREATED;
    }

    # Endpoint to get a specific consent category by ID
    #
    # + id - Unique identifier for the consent category
    # + return - returns can be any of following types 
    # http:NotFound (Consent category not found)
    resource function get consent\-categories/[string id]() returns ConsentCategory|error {
        return {
            id: id,
            category_name: "User behavior analytics",
            category_identifier: "analytics",
            purpose: "profiling",
            destinations: ["dest1", "dest2"]
        };
    }

    # Endpoint to update a consent category
    #
    # + id - Unique identifier for the consent category
    # + payload - Consent category data
    # + return - returns can be any of following types 
    # http:Ok (Consent category updated successfully)
    # http:BadRequest (Invalid consent category object)
    resource function put consent\-categories/[string id](@http:Payload ConsentCategory payload) returns http:Ok|error {
        return http:OK;
    }
}

public type Profile record {
    string profile_id;
    string origin_country;
    map<anydata> identity_attributes; // Dynamic object for identity attributes
    map<anydata> traits; // Dynamic object for traits
    anydata application_data; // Dynamic object for application data
};

public type UnificationRule record {
    string rule_id;
    string description;
    record {} conditions;
};

public type ProfileEnrichmentRule record {
    string rule_id;
    string description;
    record {} enrichment_data;
};

public type ConsentCategory record {
    string id;
    string category_name;
    string category_identifier;
    string purpose;
    string[] destinations;
};

public type Event record {
    string profile_id;
    string applicationId;
    string org_id;
    string event_type;
    string event_name;
    string event_id;
    int event_timestamp;
    map<anydata> context; // Dynamic object for context
    string locale;
    map<anydata> properties; // Dynamic object for properties
};

public type Consent record {
    string consent_id;
    string profile_id;
    string application_id;
    string category_identifier;
    boolean granted;
    string consent_channel;
    int timestamp;
    string source_ip;
    string user_agent;
};

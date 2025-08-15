/**
 * 
   
    /hypernet/manifest
    manifest system has no rigid schema beyond:
    response metadata + services field in json format
    services field can have any kind of data structure
    api users can return raw json, or render templates for plaintext or html
    beyond that requires external transforms etc.

    todo;
    need to plug in /hypernet/ request forwarding before '/' response

--------------

universal api (without php)

initial render '/' should determine instantly whether to use srcdoc for the iframe and SSR manifest
skipping entirely the intermediary PHP and writing logic in golang

replace seed() '/' reply with tree for assessing request user-agent to render the naive

removing the /hypernet logic as well


DEEP REWRITE:
Original design: HTML elements like <img> and <a> were being overloaded to encode divergent UX by dynamically resolving URLs and attributes differently for each user. This approach abused HTML semantics, made caching and debugging extremely difficult, and risked breaking future browser behavior.
Updated design: Use JSON as the canonical source of truth and generate HTML at runtime. Divergent UX is handled by a templating step that injects user-specific prefixes into href and src attributes. This keeps HTML semantic, cacheable, and maintainable while still allowing flexible, per-user UX.

--------------------

REPLACE HYPERNET.BLUE WITH HYPERCLOUD.COMPUTER AS THE API ITSELF


 * 
 */
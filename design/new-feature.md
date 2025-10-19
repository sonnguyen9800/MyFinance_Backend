I am developing a finance app that helps manage wealth, and I need your advise on how to design the database system. For the app, you don't need to talks exclusively on actual code, just discuss on theory, general ideas first. 

The requirement I am trying to make - I have various wealth, distributed in multiple form. e.g: Bank Saving Account (with interest rate), Gold, Crypto Currency, Stock, ... (more on the future) 
- First, I want to store (CRUD is enough) this data. The date I bought them, the values of them (today) 
- Then I want to know at the moment of checking, how much wealth had increased (since I aquired them). BUT because the value of currency I used to bought could be varies in values (inflation for example, I also want to be able to switch the base currency). For example 
  - ++ I bought 1 ounch of gold at 10,000,000 VNĐ in 2016, in 2025 this because 20,000,000 in 2025, which sound nice, until we reliaze that the value of 1 VND had been dropped. So I want to be able to change the base currency (for this feature only) so that I can see how much values gained (in repsective in US Dollar for example). 
  - ++ Another example: I bought stock in USD, but then the usd dropped in value, so if I want to see the increased/decresed in price IN RELATE to the gold price, ...?
- I also want to be able to register the another wealth based on: Bank Account, Cash, Physical cash, ... 
- I also want to see total wealth, the expected increased in saving book, ... Suggest a design document that can solve my requirements.
- I also want to be able to add more money to the bank account (as income from job, 2nd job, etc...)

 Ask question if required.
# Double Entry Ledger

This project is a credit card transaction flow implementation that leverages the power of Tigerbeetle, a double-entry distributed ledger database. Tigerbeetle ensures data consistency and integrity by maintaining a balanced ledger for all financial transactions.

To implement this credit card transaction flow, I utilized the Temporal library to manage distributed transactions at the application layer. This allowed for seamless integration between Tigerbeetle and the application, making it easy to manage and track financial transactions in real-time.

In addition, I used the Encore.dev framework to provide a modular and flexible codebase for the project. Encore.dev is a powerful framework that allows for rapid development and deployment of applications. With Encore.dev, I was able to easily integrate with Tigerbeetle and Temporal to implement the credit card transaction flow.

The end result is a robust and scalable system for managing credit card transactions that ensures data integrity and consistency using the Tigerbeetle double-entry distributed ledger database. The use of Temporal and Encore.dev allowed for efficient and effective implementation, making it easy to maintain and update the codebase as needed.

Overall, this project showcases my expertise in developing distributed ledger applications using Tigerbeetle and integrating with other powerful tools like Temporal and Encore.dev. If you're looking for a reliable and scalable system for managing financial transactions, don't hesitate to contact me.





## Setup guide

### Prerequisites

- install docker
- install encore
- install Go

### Run the project

- `docker-compose up -d` : This command will start the tigerbettle and temporal. 
- `encore run` : This command will start the API server.

    

You can now access the API at `http://localhost:9400/`. You can see all the API listed there. 


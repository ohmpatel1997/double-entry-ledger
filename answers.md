**1. Explain what an eventually consistent ledger would need to look out for, what are some of the CAP theorem and database considerations that are relevant when designing a bank ledger.**

- The data in an eventually consistent ledger may be temporarily inconsistent or contradictory, but it will eventually become consistent.


- CAP theory states that you can not have all the three of consistency, availability and partition tolerance when designing a distributed system. In  real world you can not avoid system from being partitioned. So in terms of a bank ledger system, consistency is critical to ensure that all users have a consistent view of the ledger, while availability is important to ensure that the system can handle high volumes of traffic and is always accessible to users. In the event of a network partition, the system must choose between being consistent and being available. Ideally its fine to choose being consistent than being available and not being consistent.


- Other important considerations I can think of a proper reconciliation system of transactions where we reconcile all the pending or invalid transactions. This is an important aspect when we try to implement the distributed transactions.


- bank ledger must ensure that data is durable and not lost due to hardware failures or other issues. This can be achieved by implementing data replication and backup strategies.

**2. Explain your solution, how does matching work? What will scaling look? 		How would you improve the API beyond a toy implementation?**

- I am maintaining a separate storage on top of the tigerbettle db ledger which keeps track of the transactions states. The reason for doing it is tigerbettle query system is not very mature has certain limitation like listing transfers of a particular account.


- When a presentment comes with an amount and account, we look for all the ‘initiated’ transactions in the external DB with given amount and debit account while taking a lock on tuples. If we find the desired transfer we update the status of the transfer to ‘in_progress’ and start the present workflow in temporal. The lock is necessary so that no other presentment can choose the same transfer.


- The above approach avoid the below scenario:
    - An auth Auth1 for account A and money amount M comes
    - An auth Auth2 for the same account A and same amount M comes.
    - Now lets say a two presentment, P1, P2 for both of the above presentments come at same time. 
    - If we don’t take a lock, both the presentment will try to match the account and amount. Both of them will matched with Auth1, sending two presentment for the same auth. Though it won’t be a problem coz tigerbettledb is fully serialized database, so one of the presentment will eventually fail. 
    - But to avoid the above situation and matching auth more efficiently I followed that approach.


- Scaling the present solution involves implementing other type of transfers as well apart from just the credit card transfers. Lets say ACH transfers. There will be two linked transfers, first from the customer ledger to bank liability ledger, second, from banks liability ledger to ach settlement ledger. Both the transfers will be link and second transfer will be in ‘pending’ state. Every night we collect all the pending transfer of the ach settlement ledger, create file and sends it over FTP. When we receive a response from clearing house, we will either settle the pending transfer or revert the transfer.

